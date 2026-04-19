package users

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/util"
)

func (r *usersRepository) QueryInsertAccount(ctx context.Context, args db.QueryInsertAccountParams) (db.QueryInsertAccountRow, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertAccount(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QuerySelectAccountByEmail(ctx context.Context, email string) (db.QuerySelectAccountByEmailRow, *repository.RepositoryError) {
	row, err := r.Q.QuerySelectAccountByEmail(ctx, email)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QuerySoftDeleteAccount(ctx context.Context, id string) *repository.RepositoryError {
	uid, appErr := util.ParseUUID(id)
	if appErr != nil {
		return appErr
	}
	return repository.HandleDatabaseError(r.Q.QuerySoftDeleteAccount(ctx, uid))
}

func (r *usersRepository) QueryUpsertProfile(ctx context.Context, args db.QueryUpsertProfileParams) (db.UserProfile, *repository.RepositoryError) {
	row, err := r.Q.QueryUpsertProfile(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QuerySelectProfile(ctx context.Context, userID string) (db.QuerySelectProfileRow, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return db.QuerySelectProfileRow{}, appErr
	}
	row, err := r.Q.QuerySelectProfile(ctx, uid)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryUpsertLinks(ctx context.Context, args db.QueryUpsertLinksParams) (db.UserLink, *repository.RepositoryError) {
	row, err := r.Q.QueryUpsertLinks(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QuerySelectAllExperiences(ctx context.Context, userID string) ([]db.UserExperience, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectAllExperiences(ctx, uid)
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QuerySelectExperiencesByTags(ctx context.Context, userID string, tags []string) ([]db.UserExperience, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectExperiencesByTags(ctx, db.QuerySelectExperiencesByTagsParams{UserID: uid, Tags: tags})
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryInsertExperience(ctx context.Context, args db.QueryInsertExperienceParams) (db.UserExperience, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertExperience(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryUpdateExperience(ctx context.Context, args db.QueryUpdateExperienceParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.QueryUpdateExperience(ctx, args))
}

func (r *usersRepository) QueryDeleteExperience(ctx context.Context, id, userID string) *repository.RepositoryError {
	eid, appErr := util.ParseUUID(id)
	if appErr != nil {
		return appErr
	}
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return appErr
	}
	return repository.HandleDatabaseError(r.Q.QueryDeleteExperience(ctx, db.QueryDeleteExperienceParams{ID: eid, UserID: uid}))
}

func (r *usersRepository) QuerySelectAllAcademicHistories(ctx context.Context, userID string) ([]db.UserAcademicHistory, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectAllAcademicHistories(ctx, uid)
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryInsertAcademicHistory(ctx context.Context, args db.QueryInsertAcademicHistoryParams) (db.UserAcademicHistory, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertAcademicHistory(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryUpdateAcademicHistory(ctx context.Context, args db.QueryUpdateAcademicHistoryParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.QueryUpdateAcademicHistory(ctx, args))
}

func (r *usersRepository) QueryDeleteAcademicHistory(ctx context.Context, id, userID string) *repository.RepositoryError {
	eid, appErr := util.ParseUUID(id)
	if appErr != nil {
		return appErr
	}
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return appErr
	}
	return repository.HandleDatabaseError(r.Q.QueryDeleteAcademicHistory(ctx, db.QueryDeleteAcademicHistoryParams{ID: eid, UserID: uid}))
}

func (r *usersRepository) QuerySelectAllSkills(ctx context.Context, userID string) ([]db.UserSkill, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectAllSkills(ctx, uid)
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QuerySelectSkillsByTags(ctx context.Context, userID string, tags []string) ([]db.UserSkill, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectSkillsByTags(ctx, db.QuerySelectSkillsByTagsParams{UserID: uid, Tags: tags})
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryInsertSkill(ctx context.Context, args db.QueryInsertSkillParams) (db.UserSkill, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertSkill(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryUpdateSkill(ctx context.Context, args db.QueryUpdateSkillParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.QueryUpdateSkill(ctx, args))
}

func (r *usersRepository) QueryDeleteSkill(ctx context.Context, id, userID string) *repository.RepositoryError {
	eid, appErr := util.ParseUUID(id)
	if appErr != nil {
		return appErr
	}
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return appErr
	}
	return repository.HandleDatabaseError(r.Q.QueryDeleteSkill(ctx, db.QueryDeleteSkillParams{ID: eid, UserID: uid}))
}

func (r *usersRepository) QuerySelectAllProjects(ctx context.Context, userID string) ([]db.UserProject, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectAllProjects(ctx, uid)
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QuerySelectProjectsByTags(ctx context.Context, userID string, tags []string) ([]db.UserProject, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectProjectsByTags(ctx, db.QuerySelectProjectsByTagsParams{UserID: uid, Tags: tags})
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryInsertProject(ctx context.Context, args db.QueryInsertProjectParams) (db.UserProject, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertProject(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryUpdateProject(ctx context.Context, args db.QueryUpdateProjectParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.QueryUpdateProject(ctx, args))
}

func (r *usersRepository) QueryDeleteProject(ctx context.Context, id, userID string) *repository.RepositoryError {
	eid, appErr := util.ParseUUID(id)
	if appErr != nil {
		return appErr
	}
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return appErr
	}
	return repository.HandleDatabaseError(r.Q.QueryDeleteProject(ctx, db.QueryDeleteProjectParams{ID: eid, UserID: uid}))
}

func (r *usersRepository) QuerySelectAllCertificates(ctx context.Context, userID string) ([]db.UserCertificate, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectAllCertificates(ctx, uid)
	return rows, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryInsertCertificate(ctx context.Context, args db.QueryInsertCertificateParams) (db.UserCertificate, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertCertificate(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *usersRepository) QueryUpdateCertificate(ctx context.Context, args db.QueryUpdateCertificateParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.QueryUpdateCertificate(ctx, args))
}

func (r *usersRepository) QueryDeleteCertificate(ctx context.Context, id, userID string) *repository.RepositoryError {
	eid, appErr := util.ParseUUID(id)
	if appErr != nil {
		return appErr
	}
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return appErr
	}
	return repository.HandleDatabaseError(r.Q.QueryDeleteCertificate(ctx, db.QueryDeleteCertificateParams{ID: eid, UserID: uid}))
}
