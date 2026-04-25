package handler

import (
	"encoding/json"
	"hackton-treino/internal/db"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Base
type Base struct {
	ID        string     `json:"id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// Auth
type Auth struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Job
type Job struct {
	Base         `json:",inline"`
	Url          string   `json:"url,omitempty"`
	CompanyName  *string  `json:"company_name,omitempty"`
	JobTitle     *string  `json:"job_title,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Stacks       []string `json:"stacks,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	Language     *string  `json:"language,omitempty"`
	Quality      *string  `json:"quality,omitempty"`
	Status       string   `json:"status,omitempty"`
}

// Resume
type Resume struct {
	Base            `json:",inline"`
	Job             *Job    `json:"job,omitempty"`
	ContentJson     string  `json:"content_json,omitempty"`
	ResumePdfPath   *string `json:"resume_pdf_path,omitempty"`
	CoverLetterPath *string `json:"cover_letter_path,omitempty"`
}

// Feedback
type FeedbackStatus string

const (
	FeedbackStatusPoor      FeedbackStatus = "poor"
	FeedbackStatusFair      FeedbackStatus = "fair"
	FeedbackStatusGood      FeedbackStatus = "good"
	FeedbackStatusExcellent FeedbackStatus = "excellent"
)

type Feedback struct {
	Base
	ResumeID string         `json:"resume_id"`
	UserID   string         `json:"user_id"`
	Status   FeedbackStatus `json:"status"`
	Comments string         `json:"comments"`
}

// Filter
type Filter struct {
	Base
	UserID  string `json:"user_id,omitempty"`
	Keyword string `json:"keyword"`
}

// User
type UserProfile struct {
	db.UserProfile
}

type Certificate struct {
	Base
	UserID      string    `json:"user_id"`
	Dominion    string    `json:"dominion"`
	Name        string    `json:"name"`
	Start       time.Time `json:"startAt"`
	End         time.Time `json:"endAt"`
	Description string    `json:"description"`
}

// request bodies

type upsertProfileReq struct {
	FullName     string  `json:"full_name"`
	Phone        *string `json:"phone"`
	About        *string `json:"about"`
	ContactEmail *string `json:"contact_email"`
}

type upsertLinksReq struct {
	LinkedinUrl  *string         `json:"linkedin_url"`
	GithubUrl    *string         `json:"github_url"`
	PortfolioUrl *string         `json:"portfolio_url"`
	OtherLinks   json.RawMessage `json:"other_links"`
}

type experienceReq struct {
	CompanyName  string   `json:"company_name"`
	JobRole      string   `json:"job_role"`
	Description  string   `json:"description"`
	IsCurrentJob bool     `json:"is_current_job"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
	TechStack    []string `json:"tech_stack"`
	Achievements []string `json:"achievements"`
	Tags         []string `json:"tags"`
}

type academicReq struct {
	InstitutionName string `json:"institution_name"`
	CourseName      string `json:"course_name"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	Description     string `json:"description"`
}

type skillReq struct {
	SkillName        string   `json:"skill_name"`
	ProficiencyLevel string   `json:"proficiency_level"`
	Tags             []string `json:"tags"`
}

type projectReq struct {
	ProjectName string   `json:"project_name"`
	Description string   `json:"description"`
	ProjectUrl  string   `json:"project_url"`
	Tags        []string `json:"tags"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	IsAcademic  bool     `json:"is_academic"`
}

type certificateReq struct {
	CertificateName     string   `json:"certificate_name"`
	IssuingOrganization string   `json:"issuing_organization"`
	IssueDate           string   `json:"issue_date"`
	CredentialUrl       string   `json:"credential_url"`
	Tags                []string `json:"tags"`
}

// profileResponse wraps QuerySelectProfileRow so OtherLinks serialises as
// a real JSON value instead of the base64 string that Go's []byte produces.
type profileResponse struct {
	Email        string          `json:"email"`
	FullName     pgtype.Text     `json:"full_name"`
	Phone        pgtype.Text     `json:"phone"`
	About        pgtype.Text     `json:"about"`
	ContactEmail pgtype.Text     `json:"contact_email"`
	LinkedinUrl  pgtype.Text     `json:"linkedin_url"`
	GithubUrl    pgtype.Text     `json:"github_url"`
	PortfolioUrl pgtype.Text     `json:"portfolio_url"`
	OtherLinks   json.RawMessage `json:"other_links"`
}
