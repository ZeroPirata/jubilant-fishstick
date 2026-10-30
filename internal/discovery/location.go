package discovery

import "strings"

// Modos de trabalho gravados pelas fontes.
var WorkplaceTypes = []string{"remote", "hybrid", "onsite"}

// Abroad é a opção "Exterior" do local de interesse: vagas de fontes internacionais.
const Abroad = "abroad"

// LocationModes são as opções aceitas nas preferências: os modos de trabalho e o exterior.
var LocationModes = append(append([]string(nil), WorkplaceTypes...), Abroad)

// onlyBrazil é o local gravado quando a vaga de uma fonte internacional aceita só
// candidatos no Brasil: na prática uma vaga remota nacional.
const onlyBrazil = "Brasil"

// LocationPrefs é o "Local de interesse" do usuário. Listas vazias não filtram.
type LocationPrefs struct {
	WorkplaceTypes []string
	Cities         []string
}

// MatchesLocation diz se a vaga entra no local de interesse. Vaga internacional só
// depende de "Exterior" estar marcado. Nas nacionais, as cidades valem só para
// híbrida ou presencial: remota passa de qualquer lugar. Vaga sem modo informado só
// passa se não houver filtro de cidade ou se a cidade bater.
func (p LocationPrefs) MatchesLocation(workplaceType, location string, international bool) bool {
	if len(p.WorkplaceTypes) == 0 {
		return international || p.matchesCity(workplaceType, location)
	}
	if international {
		return contains(p.WorkplaceTypes, Abroad)
	}
	modes := make([]string, 0, len(p.WorkplaceTypes))
	for _, m := range p.WorkplaceTypes {
		if m != Abroad {
			modes = append(modes, m)
		}
	}
	if len(modes) == 0 {
		return false // marcou só "Exterior"
	}
	if workplaceType != "" && !contains(modes, workplaceType) {
		return false
	}
	return p.matchesCity(workplaceType, location)
}

func (p LocationPrefs) matchesCity(workplaceType, location string) bool {
	if workplaceType == "remote" || len(p.Cities) == 0 {
		return true
	}
	loc := foldAccents(strings.ToLower(location))
	for _, c := range p.Cities {
		if c = foldAccents(strings.ToLower(strings.TrimSpace(c))); c != "" && strings.Contains(loc, c) {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// "sao jose dos campos" bate em "São José dos Campos".
var accentFolder = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "è", "e",
	"í", "i", "î", "i",
	"ó", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ü", "u",
	"ç", "c",
)

func foldAccents(s string) string { return accentFolder.Replace(s) }

// Marcadores de vaga no Brasil e de vaga remota aberta à América Latina / ao mundo,
// já em minúsculas e sem acento (comparados com foldAccents).
var (
	brazilMarkers = []string{
		"brazil", "brasil", "sao paulo", "rio de janeiro", "belo horizonte", "curitiba",
		"porto alegre", "florianopolis", "campinas", "recife", "salvador", "fortaleza",
		"brasilia", "goiania", "joinville", "sao jose dos campos", "barueri", "osasco",
		"ribeirao preto", "uberlandia", "blumenau", "manaus", "belem", "vitoria",
	}
	openRemoteMarkers = []string{
		"latam", "latin america", "south america", "americas", "worldwide", "anywhere", "global",
	}
	remoteMarkers = []string{"remote", "remoto", "home office", "anywhere"}
	hybridMarkers = []string{"hybrid", "hibrido"}
)

// ClassifyPlace decide, pelo texto de local de um ATS, o modo de trabalho e se a vaga
// é do exterior. keep=false descarta a vaga: fora do Brasil e não remota para cá
// (escritório no México, "Remote - US").
func ClassifyPlace(location string, remoteHint bool) (workplace string, international, keep bool) {
	loc := foldAccents(strings.ToLower(location))
	remote := remoteHint || hasAny(loc, remoteMarkers)
	switch {
	case hasAny(loc, hybridMarkers):
		workplace = "hybrid"
	case remote:
		workplace = "remote"
	}
	if hasAny(loc, brazilMarkers) {
		return workplace, false, true
	}
	if remote && hasAny(loc, openRemoteMarkers) {
		return "remote", true, true
	}
	return "", false, false
}

// hasAny procura os marcadores como palavra inteira ("sp" não bate em "spain").
func hasAny(text string, markers []string) bool {
	for _, m := range markers {
		if containsWord(text, m) {
			return true
		}
	}
	return false
}
