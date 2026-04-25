package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"go.uber.org/zap"
)

func resolvePython() string {
	for _, candidate := range []string{"/usr/bin/python3", "/usr/local/bin/python3", "python3"} {
		if p, err := exec.LookPath(candidate); err == nil {
			return p
		}
	}
	return "python3"
}

// PDFService mantém um processo Python persistente para gerar PDFs.
// Elimina o cold start do WeasyPrint (~1-3s) que ocorre a cada exec.Command.
type PDFService struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	logger  *zap.Logger
}

func NewPDFService(logger *zap.Logger) *PDFService {
	s := &PDFService{logger: logger}
	s.start()
	return s
}

func (s *PDFService) start() {
	python := resolvePython()
	// -u desativa o buffer interno do Python — essencial para o protocolo linha-a-linha.
	cmd := exec.Command(python, "-u", "scripts/gerar_pdf.py", "--serve")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.logger.Error("pdf service: stdin pipe", zap.Error(err))
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.logger.Error("pdf service: stdout pipe", zap.Error(err))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.logger.Error("pdf service: stderr pipe", zap.Error(err))
		return
	}

	if err := cmd.Start(); err != nil {
		s.logger.Error("pdf service: start python", zap.Error(err))
		return
	}

	s.cmd = cmd
	s.stdin = stdin
	s.scanner = bufio.NewScanner(stdout)
	s.logger.Info("pdf service: processo python iniciado", zap.Int("pid", cmd.Process.Pid))

	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			s.logger.Warn("pdf service: python stderr", zap.String("line", sc.Text()))
		}
	}()
}

func (s *PDFService) restart() {
	s.logger.Warn("pdf service: reiniciando processo python")
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait() //nolint:errcheck
	}
	s.start()
}

// Generate envia um request ao processo Python e aguarda a resposta.
// É thread-safe e respeita o context deadline.
func (s *PDFService) Generate(ctx context.Context, input pythonInput) (*pythonOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd == nil {
		return nil, fmt.Errorf("pdf service: processo python não iniciado")
	}

	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("pdf service: marshal: %w", err)
	}

	if _, err := fmt.Fprintf(s.stdin, "%s\n", data); err != nil {
		s.restart()
		if s.stdin == nil {
			return nil, fmt.Errorf("pdf service: write falhou e restart não iniciou processo: %w", err)
		}
		if _, err2 := fmt.Fprintf(s.stdin, "%s\n", data); err2 != nil {
			return nil, fmt.Errorf("pdf service: write falhou após restart: %w", err2)
		}
	}

	type scanResult struct {
		line []byte
		err  error
	}
	ch := make(chan scanResult, 1)
	scanner := s.scanner // captura após possível restart acima
	go func() {
		if scanner.Scan() {
			b := make([]byte, len(scanner.Bytes()))
			copy(b, scanner.Bytes())
			ch <- scanResult{line: b}
		} else {
			readErr := scanner.Err()
			if readErr == nil {
				readErr = io.EOF
			}
			ch <- scanResult{err: readErr}
		}
	}()

	select {
	case <-ctx.Done():
		// Mata o processo para desbloquear a goroutine acima (que vai receber EOF
		// e enviar para o canal bufferizado sem bloquear), depois reinicia.
		s.restart()
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			s.restart()
			return nil, fmt.Errorf("pdf service: read response: %w", r.err)
		}
		var out pythonOutput
		if err := json.Unmarshal(r.line, &out); err != nil {
			return nil, fmt.Errorf("pdf service: parse response: %w", err)
		}
		return &out, nil
	}
}
