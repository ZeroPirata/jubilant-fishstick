package worker

type vagaPrompt struct {
	Empresa    string   `json:"empresa"`
	Titulo     string   `json:"titulo"`
	Descricao  string   `json:"descricao,omitempty"`
	Stack      []string `json:"stack,omitempty"`
	Requisitos []string `json:"requisitos,omitempty"`
}

type experienciaPrompt struct {
	Empresa    string   `json:"empresa"`
	Cargo      string   `json:"cargo"`
	Descricao  string   `json:"descricao,omitempty"`
	DataInicio string   `json:"data_inicio,omitempty"`
	DataFim    string   `json:"data_fim,omitempty"`
	Atual      bool     `json:"atual"`
	Stack      []string `json:"stack"`
	Conquistas []string `json:"conquistas"`
}

type habilidadePrompt struct {
	Nome  string `json:"nome"`
	Nivel string `json:"nivel"`
}

type projetoPrompt struct {
	Nome      string `json:"nome"`
	Descricao string `json:"descricao,omitempty"`
	Link      string `json:"link,omitempty"`
}

type formacaoPrompt struct {
	Instituicao string `json:"instituicao"`
	Curso       string `json:"curso"`
	DataInicio  string `json:"data_inicio,omitempty"`
	DataFim     string `json:"data_fim,omitempty"`
}

type certificacaoPrompt struct {
	Nome      string `json:"nome"`
	Emissor   string `json:"emissor"`
	EmitidoEm string `json:"emitido_em,omitempty"`
	Link      string `json:"link,omitempty"`
}

type feedbackPrompt struct {
	ExemplosExcelentes    []string `json:"exemplos_excelentes"`
	ComentariosAnteriores []string `json:"comentarios_anteriores"`
}

type userPrompt struct {
	Vaga          vagaPrompt           `json:"vaga"`
	Experiencias  []experienciaPrompt  `json:"experiencias"`
	Habilidades   []habilidadePrompt   `json:"habilidades,omitempty"`
	Projetos      []projetoPrompt      `json:"projetos,omitempty"`
	Formacoes     []formacaoPrompt     `json:"formacoes,omitempty"`
	Certificacoes []certificacaoPrompt `json:"certificacoes,omitempty"`
	Feedback      feedbackPrompt       `json:"feedback"`
	Modo          string               `json:"modo,omitempty"` // "resume_only" | "cover_only" — ausente = full
}
