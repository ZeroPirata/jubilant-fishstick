package handler

import (
	"testing"
)

// =============================================================================
// Required / MinLength / MaxLength / ValidateEmail
// =============================================================================

func TestRequired(t *testing.T) {
	cases := []struct {
		label   string
		value   string
		wantErr bool
	}{
		{"valor presente", "gabriel", false},
		{"string vazia", "", true},
		{"so espacos", "   ", true},
		{"espaco com conteudo", " gabriel ", false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			err := Required("campo", tc.value)()
			if (err != nil) != tc.wantErr {
				t.Errorf("Required(%q) erro = %v, wantErr = %v", tc.value, err, tc.wantErr)
			}
		})
	}
}

func TestMinLength(t *testing.T) {
	cases := []struct {
		label   string
		value   string
		min     int
		wantErr bool
	}{
		{"exatamente no minimo", "abc", 3, false},
		{"acima do minimo", "abcd", 3, false},
		{"abaixo do minimo", "ab", 3, true},
		{"vazio quando min=0", "", 0, false},
		{"vazio quando min=1", "", 1, true},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			err := MinLength("campo", tc.value, tc.min)()
			if (err != nil) != tc.wantErr {
				t.Errorf("MinLength(%q, %d) erro = %v, wantErr = %v", tc.value, tc.min, err, tc.wantErr)
			}
		})
	}
}

func TestMaxLength(t *testing.T) {
	cases := []struct {
		label   string
		value   string
		max     int
		wantErr bool
	}{
		{"exatamente no maximo", "abc", 3, false},
		{"abaixo do maximo", "ab", 3, false},
		{"acima do maximo", "abcd", 3, true},
		{"vazio sempre ok", "", 3, false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			err := MaxLength("campo", tc.value, tc.max)()
			if (err != nil) != tc.wantErr {
				t.Errorf("MaxLength(%q, %d) erro = %v, wantErr = %v", tc.value, tc.max, err, tc.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		label   string
		value   string
		wantErr bool
	}{
		{"email valido simples", "user@example.com", false},
		{"email valido com subdominio", "user@mail.example.com", false},
		{"sem arroba", "userexample.com", true},
		{"sem ponto", "user@example", true},
		{"vazio", "", true},
		// a validação atual é minimalista (@ + ponto), não um parser RFC completo
		{"arroba e ponto presentes", "a@b.c", false},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			err := ValidateEmail("email", tc.value)()
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateEmail(%q) erro = %v, wantErr = %v", tc.value, err, tc.wantErr)
			}
		})
	}
}

// =============================================================================
// Validate (combina validators)
// =============================================================================

func TestValidate_CurtocircuitoNoPrimeiroErro(t *testing.T) {
	// Validate deve parar no primeiro validator que falha
	var chamados []string

	v1 := func() error {
		chamados = append(chamados, "v1")
		return nil
	}
	v2 := func() error {
		chamados = append(chamados, "v2")
		return Required("campo", "")() // falha aqui
	}
	v3 := func() error {
		chamados = append(chamados, "v3")
		return nil
	}

	err := Validate(v1, v2, v3)

	if err == nil {
		t.Fatal("esperava erro do v2, recebeu nil")
	}
	if len(chamados) != 2 {
		t.Errorf("chamados = %v, esperava [v1 v2] (v3 não deve ser chamado)", chamados)
	}
}

func TestValidate_TodosPassam(t *testing.T) {
	err := Validate(
		Required("nome", "gabriel"),
		MinLength("senha", "minhasenha", 8),
		MaxLength("bio", "curta", 100),
		ValidateEmail("email", "gabriel@email.com"),
	)
	if err != nil {
		t.Errorf("Validate não deveria retornar erro: %v", err)
	}
}

// =============================================================================
// TypedRequired
// =============================================================================

type registerBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func TestTypedRequired_CamposPresentes(t *testing.T) {
	validate := TypedRequired[registerBody]("email", "password", "name")

	body := registerBody{
		Email:    "user@example.com",
		Password: "minha-senha",
		Name:     "Gabriel",
	}

	if err := validate(body); err != nil {
		t.Errorf("TypedRequired: erro inesperado: %v", err)
	}
}

func TestTypedRequired_CampoVazio(t *testing.T) {
	validate := TypedRequired[registerBody]("email", "password")

	// email vazio → erro
	body := registerBody{Password: "minha-senha"}

	if err := validate(body); err == nil {
		t.Error("TypedRequired: esperava erro para campo email vazio")
	}
}

func TestTypedRequired_CampoInexistente(t *testing.T) {
	// Campo que não existe no struct → erro "field not found"
	validate := TypedRequired[registerBody]("nao_existe")

	if err := validate(registerBody{}); err == nil {
		t.Error("TypedRequired: esperava erro para campo inexistente")
	}
}

func TestTypedRequired_BuscaPorJsonTag(t *testing.T) {
	// TypedRequired aceita tanto o json tag ("email") quanto o nome do campo ("Email")
	byTag := TypedRequired[registerBody]("email")
	byName := TypedRequired[registerBody]("Email")

	body := registerBody{Email: "user@example.com", Password: "senha", Name: "X"}

	if err := byTag(body); err != nil {
		t.Errorf("busca por json tag falhou: %v", err)
	}
	if err := byName(body); err != nil {
		t.Errorf("busca por nome do campo falhou: %v", err)
	}
}

func TestTypedValidate_CombinandoValidators(t *testing.T) {
	// TypedValidate compõe múltiplos TypedValidator[T] em um único
	validate := TypedValidate[registerBody](
		TypedRequired[registerBody]("email", "password"),
		func(b registerBody) error {
			return MinLength("password", b.Password, 8)()
		},
	)

	t.Run("tudo valido", func(t *testing.T) {
		err := validate(registerBody{Email: "a@b.com", Password: "senha-longa", Name: "X"})
		if err != nil {
			t.Errorf("erro inesperado: %v", err)
		}
	})

	t.Run("senha curta demais", func(t *testing.T) {
		err := validate(registerBody{Email: "a@b.com", Password: "curta", Name: "X"})
		if err == nil {
			t.Error("esperava erro para senha curta")
		}
	})

	t.Run("email vazio", func(t *testing.T) {
		err := validate(registerBody{Email: "", Password: "senha-longa", Name: "X"})
		if err == nil {
			t.Error("esperava erro para email vazio")
		}
	})
}
