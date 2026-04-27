package worker

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/metrics"
	"hackton-treino/internal/repository"
	workerRepo "hackton-treino/internal/repository/worker"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

// fakeWorkerRepo é um stub do workerRepo.Repository para testes.
// Apenas WorkerRecoverStuckJobs e WorkerSelectPendingJobs são implementados;
// os demais métodos não são chamados por recoverStuckJobs.
type fakeWorkerRepo struct {
	recoverCalled bool
	recoverCutoff pgtype.Timestamptz
	recoverReturn int64
	recoverErr    *repository.RepositoryError
}

func (f *fakeWorkerRepo) WorkerRecoverStuckJobs(ctx context.Context, cutoff pgtype.Timestamptz) (int64, *repository.RepositoryError) {
	f.recoverCalled = true
	f.recoverCutoff = cutoff
	return f.recoverReturn, f.recoverErr
}

func (f *fakeWorkerRepo) WorkerSelectPendingJobs(_ context.Context, _ int32) ([]db.Job, *repository.RepositoryError) {
	return nil, nil
}
func (f *fakeWorkerRepo) WorkerUpdateJobStatus(_ context.Context, _ db.WorkerUpdateJobStatusParams) *repository.RepositoryError {
	return nil
}
func (f *fakeWorkerRepo) WorkerUpdateJob(_ context.Context, _ db.WorkerUpdateJobParams) *repository.RepositoryError {
	return nil
}
func (f *fakeWorkerRepo) WorkerUpdateJobQuality(_ context.Context, _ db.WorkerUpdateJobQualityParams) *repository.RepositoryError {
	return nil
}
func (f *fakeWorkerRepo) WorkerInsertGeneratedResume(_ context.Context, _ db.WorkerInsertGeneratedResumeParams) (string, *repository.RepositoryError) {
	return "", nil
}

// compile-time: garante que fakeWorkerRepo implementa a interface completa.
var _ workerRepo.Repository = (*fakeWorkerRepo)(nil)

func newTestWorker(repo workerRepo.Repository, timeout time.Duration) *Worker {
	return &Worker{
		Logger:          zap.NewNop(),
		Pipeline:        repo,
		recoveryTimeout: timeout,
	}
}

func TestRecoverStuckJobs_SkipsWhenTimeoutZero(t *testing.T) {
	fake := &fakeWorkerRepo{}
	w := newTestWorker(fake, 0)

	w.recoverStuckJobs(context.Background())

	if fake.recoverCalled {
		t.Error("esperava que WorkerRecoverStuckJobs não fosse chamado com timeout=0")
	}
}

func TestRecoverStuckJobs_CallsRepoWithCorrectCutoff(t *testing.T) {
	fake := &fakeWorkerRepo{recoverReturn: 0}
	timeout := 10 * time.Minute
	w := newTestWorker(fake, timeout)

	before := time.Now()
	w.recoverStuckJobs(context.Background())
	after := time.Now()

	if !fake.recoverCalled {
		t.Fatal("WorkerRecoverStuckJobs não foi chamado")
	}

	// O cutoff deve ser aproximadamente time.Now() - timeout.
	expectedMin := before.Add(-timeout)
	expectedMax := after.Add(-timeout)
	cutoffTime := fake.recoverCutoff.Time

	if cutoffTime.Before(expectedMin) || cutoffTime.After(expectedMax) {
		t.Errorf("cutoff %v fora do intervalo esperado [%v, %v]", cutoffTime, expectedMin, expectedMax)
	}
}

func TestRecoverStuckJobs_IncrementsMetricOnRecovery(t *testing.T) {
	const recovered int64 = 3
	fake := &fakeWorkerRepo{recoverReturn: recovered}
	w := newTestWorker(fake, 10*time.Minute)

	before := testutil.ToFloat64(metrics.JobsProcessed.WithLabelValues("recovered"))
	w.recoverStuckJobs(context.Background())
	after := testutil.ToFloat64(metrics.JobsProcessed.WithLabelValues("recovered"))

	if delta := after - before; delta != float64(recovered) {
		t.Errorf("métrica recovered: delta esperado %d, obtido %v", recovered, delta)
	}
}

func TestRecoverStuckJobs_HandlesRepoError(t *testing.T) {
	fake := &fakeWorkerRepo{
		recoverErr: &repository.RepositoryError{StatusCode: 500, Message: "db error"},
	}
	w := newTestWorker(fake, 10*time.Minute)

	// Não deve entrar em pânico; apenas loga o erro.
	w.recoverStuckJobs(context.Background())

	if !fake.recoverCalled {
		t.Error("WorkerRecoverStuckJobs deveria ter sido chamado mesmo com erro")
	}
}

func TestRecoverStuckJobs_NoMetricIncrementOnZeroRecovered(t *testing.T) {
	fake := &fakeWorkerRepo{recoverReturn: 0}
	w := newTestWorker(fake, 10*time.Minute)

	before := testutil.ToFloat64(metrics.JobsProcessed.WithLabelValues("recovered"))
	w.recoverStuckJobs(context.Background())
	after := testutil.ToFloat64(metrics.JobsProcessed.WithLabelValues("recovered"))

	if before != after {
		t.Errorf("métrica não deveria ter sido incrementada com 0 jobs recuperados")
	}
}
