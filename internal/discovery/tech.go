package discovery

import "strings"

// Technology é uma linguagem, framework ou ferramenta que o usuário escolhe em
// "Tecnologias de interesse". Ela filtra a lista (vagas que a citam, mesmo fora do
// perfil) e entra nas buscas dos sites com o termo Search.
type Technology struct {
	Name   string   `json:"name"`
	Group  string   `json:"group"`
	Terms  []string `json:"terms"`  // como aparece na vaga, em minúsculas
	Search string   `json:"search"` // termo usado na busca dos sites
}

func tech(group, name, search string, terms ...string) Technology {
	return Technology{Name: name, Group: group, Terms: terms, Search: search}
}

// Technologies é o catálogo, na ordem em que aparece na tela.
var Technologies = []Technology{
	// "go" sozinho só conta com contexto de tecnologia (ambiguousTerms).
	tech("Linguagens", "Go", "golang", "golang", "go"),
	tech("Linguagens", "Python", "python", "python"),
	tech("Linguagens", "Java", "java", "java"),
	tech("Linguagens", "JavaScript", "javascript", "javascript", "js"),
	tech("Linguagens", "TypeScript", "typescript", "typescript", "ts"),
	tech("Linguagens", "C#", "c#", "c#", "csharp"),
	tech("Linguagens", "C++", "c++", "c++", "cpp"),
	tech("Linguagens", "PHP", "php", "php"),
	tech("Linguagens", "Ruby", "ruby", "ruby"),
	tech("Linguagens", "Kotlin", "kotlin", "kotlin"),
	tech("Linguagens", "Swift", "swift", "swift"),
	tech("Linguagens", "Rust", "rust", "rust"),
	tech("Linguagens", "Scala", "scala", "scala"),
	tech("Linguagens", "Elixir", "elixir", "elixir"),
	tech("Linguagens", "Dart", "dart", "dart"),
	tech("Linguagens", "SQL", "sql", "sql"),

	tech("Front-end", "React", "react", "react", "react.js", "reactjs"),
	tech("Front-end", "Angular", "angular", "angular"),
	tech("Front-end", "Vue.js", "vue", "vue", "vue.js", "vuejs"),
	tech("Front-end", "Next.js", "next.js", "next.js", "nextjs"),
	tech("Front-end", "HTML e CSS", "html", "html", "css"),
	tech("Front-end", "Tailwind", "tailwind", "tailwind", "tailwindcss"),

	tech("Back-end", "Node.js", "node.js", "node.js", "nodejs", "node"),
	tech("Back-end", ".NET", ".net", ".net", "dotnet", "asp.net"),
	tech("Back-end", "Spring", "spring boot", "spring boot", "spring framework", "spring"),
	tech("Back-end", "Django", "django", "django"),
	tech("Back-end", "FastAPI", "fastapi", "fastapi"),
	tech("Back-end", "Flask", "flask", "flask"),
	tech("Back-end", "Laravel", "laravel", "laravel"),
	tech("Back-end", "Ruby on Rails", "rails", "ruby on rails", "rails"),
	tech("Back-end", "NestJS", "nestjs", "nestjs", "nest.js"),
	tech("Back-end", "GraphQL", "graphql", "graphql"),
	tech("Back-end", "gRPC", "grpc", "grpc"),
	tech("Back-end", "Microsserviços", "microsserviços", "microsserviços", "microsserviço", "microservices", "microservice"),

	tech("Mobile", "Flutter", "flutter", "flutter"),
	tech("Mobile", "React Native", "react native", "react native"),
	tech("Mobile", "Android", "android", "android"),
	tech("Mobile", "iOS", "ios", "ios"),

	tech("Dados e IA", "Spark", "spark", "spark", "pyspark"),
	tech("Dados e IA", "Airflow", "airflow", "airflow"),
	tech("Dados e IA", "Pandas", "pandas", "pandas"),
	tech("Dados e IA", "Power BI", "power bi", "power bi", "powerbi"),
	tech("Dados e IA", "Tableau", "tableau", "tableau"),
	tech("Dados e IA", "Databricks", "databricks", "databricks"),
	tech("Dados e IA", "Machine Learning", "machine learning", "machine learning", "aprendizado de máquina"),
	tech("Dados e IA", "LLM / IA generativa", "llm", "llm", "llms", "genai", "ia generativa", "generative ai"),
	tech("Dados e IA", "TensorFlow", "tensorflow", "tensorflow"),
	tech("Dados e IA", "PyTorch", "pytorch", "pytorch"),

	tech("Bancos de dados", "PostgreSQL", "postgresql", "postgresql", "postgres"),
	tech("Bancos de dados", "MySQL", "mysql", "mysql"),
	tech("Bancos de dados", "SQL Server", "sql server", "sql server"),
	tech("Bancos de dados", "Oracle", "oracle", "oracle"),
	tech("Bancos de dados", "MongoDB", "mongodb", "mongodb", "mongo"),
	tech("Bancos de dados", "Redis", "redis", "redis"),
	tech("Bancos de dados", "Elasticsearch", "elasticsearch", "elasticsearch"),
	tech("Bancos de dados", "DynamoDB", "dynamodb", "dynamodb"),

	tech("Cloud e DevOps", "AWS", "aws", "aws", "amazon web services"),
	tech("Cloud e DevOps", "Azure", "azure", "azure"),
	tech("Cloud e DevOps", "Google Cloud", "gcp", "gcp", "google cloud"),
	tech("Cloud e DevOps", "Docker", "docker", "docker"),
	tech("Cloud e DevOps", "Kubernetes", "kubernetes", "kubernetes", "k8s"),
	tech("Cloud e DevOps", "Terraform", "terraform", "terraform"),
	tech("Cloud e DevOps", "CI/CD", "ci/cd", "ci/cd", "github actions", "gitlab ci", "jenkins"),
	tech("Cloud e DevOps", "Linux", "linux", "linux"),
	tech("Cloud e DevOps", "Kafka", "kafka", "kafka"),
	tech("Cloud e DevOps", "RabbitMQ", "rabbitmq", "rabbitmq"),

	tech("Testes", "Cypress", "cypress", "cypress"),
	tech("Testes", "Selenium", "selenium", "selenium"),
	tech("Testes", "Playwright", "playwright", "playwright"),
	tech("Testes", "Jest", "jest", "jest"),

	tech("Ferramentas", "Figma", "figma", "figma"),
	tech("Ferramentas", "Jira", "jira", "jira"),
	tech("Ferramentas", "SAP", "sap", "sap"),
	tech("Ferramentas", "Salesforce", "salesforce", "salesforce"),
	tech("Ferramentas", "Excel", "excel", "excel"),
	tech("Ferramentas", "Git", "git", "git"),
}

var techByName = func() map[string]Technology {
	m := make(map[string]Technology, len(Technologies))
	for _, t := range Technologies {
		m[t.Name] = t
	}
	return m
}()

// TechByName acha a tecnologia pelo nome exibido.
func TechByName(name string) (Technology, bool) {
	t, ok := techByName[name]
	return t, ok
}

// TechSkills monta os termos das tecnologias no formato do casamento de skills (para
// o regex do banco). O casamento final usa MatchTechs, que exige contexto nos termos ambíguos.
func TechSkills(names []string) []Skill {
	var out []Skill
	for _, n := range names {
		if t, ok := TechByName(n); ok {
			out = append(out, Skill{Name: t.Name, Terms: t.Terms})
		}
	}
	return out
}

// MatchTechs devolve as tecnologias citadas no texto da vaga, como palavra inteira.
func MatchTechs(text string, names []string) []string {
	lower := strings.ToLower(text)
	var matched []string
	for _, n := range names {
		t, ok := TechByName(n)
		if !ok {
			continue
		}
		for _, term := range t.Terms {
			if containsTerm(lower, text, term) {
				matched = append(matched, t.Name)
				break
			}
		}
	}
	return matched
}

// TechNames são os nomes de todo o catálogo, para achar a stack inteira da vaga.
var TechNames = func() []string {
	out := make([]string, len(Technologies))
	for i, t := range Technologies {
		out[i] = t.Name
	}
	return out
}()

// TechCoveredBy diz se a tecnologia já está representada por uma skill do perfil
// (skill "Go (Golang)" cobre a tecnologia Go): os termos se cruzam.
func TechCoveredBy(name string, skill Skill) bool {
	t, ok := TechByName(name)
	if !ok {
		return false
	}
	for _, a := range t.Terms {
		for _, b := range skill.Terms {
			if a == b {
				return true
			}
		}
	}
	return false
}

// TechSearchTerms são os termos das tecnologias para buscar nos sites.
func TechSearchTerms(names []string) []string {
	var out []string
	for _, n := range names {
		if t, ok := TechByName(n); ok {
			out = append(out, t.Search)
		}
	}
	return out
}
