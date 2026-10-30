package scraper

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Leitores dos ATS que têm API pública por vaga: mais limpo que o HTML (Greenhouse,
// SmartRecruiters) ou a única saída quando a página é montada por JavaScript (Workable).

var (
	// job-boards.greenhouse.io/<token>/jobs/<id>, boards.greenhouse.io/<token>/jobs/<id>
	greenhousePath = regexp.MustCompile(`^/([a-z0-9._-]+)/jobs/(\d+)`)
	// jobs.smartrecruiters.com/<empresa>/<id>-<slug>
	smartRecruitersPath = regexp.MustCompile(`^/([A-Za-z0-9._-]+)/(\d+)`)
	// apply.workable.com/<conta>/j/<shortcode>
	workablePath = regexp.MustCompile(`^/([a-z0-9._-]+)/j/([A-Za-z0-9]+)`)
)

func getJSON(url string, out any) error {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", readerUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("a vaga não existe mais (status 404)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *Scraper) scrapGreenhouse(u *url.URL) (BasicScraperResult, error) {
	m := greenhousePath.FindStringSubmatch(strings.ToLower(u.Path))
	if m == nil {
		return s.scrapGeneric()
	}
	var job struct {
		Title       string `json:"title"`
		CompanyName string `json:"company_name"`
		Content     string `json:"content"`
	}
	if err := getJSON(fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs/%s", m[1], m[2]), &job); err != nil {
		return BasicScraperResult{}, fmt.Errorf("Greenhouse: %w", err)
	}
	return BasicScraperResult{
		Title:            strings.TrimSpace(job.Title),
		Company:          strings.TrimSpace(job.CompanyName),
		BasicDescription: HTMLToText(html.UnescapeString(job.Content)), // vem escapado
	}, nil
}

func (s *Scraper) scrapSmartRecruiters(u *url.URL) (BasicScraperResult, error) {
	m := smartRecruitersPath.FindStringSubmatch(u.Path)
	if m == nil {
		return s.scrapGeneric()
	}
	var job struct {
		Name    string `json:"name"`
		Company struct {
			Name string `json:"name"`
		} `json:"company"`
		JobAd struct {
			Sections map[string]struct {
				Title string `json:"title"`
				Text  string `json:"text"`
			} `json:"sections"`
		} `json:"jobAd"`
	}
	if err := getJSON(fmt.Sprintf("https://api.smartrecruiters.com/v1/companies/%s/postings/%s", m[1], m[2]), &job); err != nil {
		return BasicScraperResult{}, fmt.Errorf("SmartRecruiters: %w", err)
	}
	var parts []string
	for _, key := range []string{"jobDescription", "qualifications", "additionalInformation"} {
		if sec, ok := job.JobAd.Sections[key]; ok {
			if text := HTMLToText(sec.Text); text != "" {
				parts = append(parts, strings.TrimSpace(sec.Title+"\n"+text))
			}
		}
	}
	return BasicScraperResult{
		Title:            strings.TrimSpace(job.Name),
		Company:          strings.TrimSpace(job.Company.Name),
		BasicDescription: strings.Join(parts, "\n\n"),
	}, nil
}

func (s *Scraper) scrapWorkable(u *url.URL) (BasicScraperResult, error) {
	m := workablePath.FindStringSubmatch(u.Path)
	if m == nil {
		return BasicScraperResult{}, ErrNeedsBrowser
	}
	var job struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		Requirements string `json:"requirements"`
		Benefits     string `json:"benefits"`
	}
	if err := getJSON(fmt.Sprintf("https://apply.workable.com/api/v2/accounts/%s/jobs/%s", m[1], m[2]), &job); err != nil {
		return BasicScraperResult{}, fmt.Errorf("Workable: %w", err)
	}
	var parts []string
	for _, section := range []string{job.Description, job.Requirements, job.Benefits} {
		if text := HTMLToText(section); text != "" {
			parts = append(parts, text)
		}
	}
	return BasicScraperResult{
		Title: strings.TrimSpace(job.Title),
		// A API da vaga não traz o nome da empresa; a conta é o melhor que há.
		Company:          m[1],
		BasicDescription: strings.Join(parts, "\n\n"),
	}, nil
}
