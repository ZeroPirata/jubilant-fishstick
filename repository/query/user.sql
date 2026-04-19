-- name: QueryInsertAccount :one
INSERT INTO user_accounts(email, password_hash)
VALUES (@email, @password_hash)
RETURNING id, email, created_at;

-- name: QuerySelectAccountByEmail :one
SELECT id, email, password_hash, deleted_at FROM user_accounts
WHERE email = @email AND deleted_at IS NULL;

-- name: QuerySoftDeleteAccount :exec
UPDATE user_accounts
SET deleted_at = now()
WHERE id = @id;

-- name: QueryUpsertProfile :one
INSERT INTO user_profiles(user_id, full_name, phone, about)
VALUES (@user_id, @full_name, @phone, @about)
ON CONFLICT (user_id) DO UPDATE SET
    full_name  = EXCLUDED.full_name,
    phone      = EXCLUDED.phone,
    about      = EXCLUDED.about,
    updated_at = now()
RETURNING *;

-- name: QuerySelectProfile :one
SELECT
    acc.email,
    p.full_name, p.phone, p.about,
    l.linkedin_url, l.github_url, l.portfolio_url, l.other_links
FROM user_accounts acc
LEFT JOIN user_profiles p ON p.user_id = acc.id
LEFT JOIN user_links l ON l.user_id = acc.id
WHERE acc.id = @user_id AND acc.deleted_at IS NULL;

-- name: QueryUpsertLinks :one
INSERT INTO user_links(user_id, linkedin_url, github_url, portfolio_url, other_links)
VALUES (@user_id, @linkedin_url, @github_url, @portfolio_url, @other_links)
ON CONFLICT (user_id) DO UPDATE SET
    linkedin_url  = EXCLUDED.linkedin_url,
    github_url    = EXCLUDED.github_url,
    portfolio_url = EXCLUDED.portfolio_url,
    other_links   = EXCLUDED.other_links,
    updated_at    = now()
RETURNING *;

-- name: QuerySelectAllExperiences :many
SELECT * FROM user_experiences
WHERE user_id = @user_id AND deleted_at IS NULL
ORDER BY start_date DESC;

-- name: QuerySelectExperiencesByTags :many
SELECT * FROM user_experiences
WHERE user_id = @user_id AND tags && @tags::TEXT[] AND deleted_at IS NULL
ORDER BY start_date DESC;

-- name: QueryInsertExperience :one
INSERT INTO user_experiences(
    user_id, company_name, job_role, description,
    is_current_job, start_date, end_date, tech_stack, achievements, tags
) VALUES (
    @user_id, @company_name, @job_role, @description,
    @is_current_job, @start_date, @end_date, @tech_stack, @achievements, @tags
) RETURNING *;

-- name: QueryUpdateExperience :exec
UPDATE user_experiences
SET
    company_name   = @company_name,
    job_role       = @job_role,
    description    = @description,
    is_current_job = @is_current_job,
    start_date     = @start_date,
    end_date       = @end_date,
    tech_stack     = @tech_stack,
    achievements   = @achievements,
    tags           = @tags,
    updated_at     = now()
WHERE id = @id AND user_id = @user_id;

-- name: QueryDeleteExperience :exec
UPDATE user_experiences
SET deleted_at = now()
WHERE id = @id AND user_id = @user_id;

-- name: QuerySelectAllAcademicHistories :many
SELECT * FROM user_academic_histories
WHERE user_id = @user_id AND deleted_at IS NULL
ORDER BY start_date DESC;

-- name: QueryInsertAcademicHistory :one
INSERT INTO user_academic_histories(
    user_id, institution_name, course_name, start_date, end_date, description
) VALUES (
    @user_id, @institution_name, @course_name, @start_date, @end_date, @description
) RETURNING *;

-- name: QueryUpdateAcademicHistory :exec
UPDATE user_academic_histories
SET
    institution_name = @institution_name,
    course_name      = @course_name,
    start_date       = @start_date,
    end_date         = @end_date,
    description      = @description,
    updated_at       = now()
WHERE id = @id AND user_id = @user_id;

-- name: QueryDeleteAcademicHistory :exec
UPDATE user_academic_histories
SET deleted_at = now()
WHERE id = @id AND user_id = @user_id;

-- name: QuerySelectAllSkills :many
SELECT * FROM user_skills
WHERE user_id = @user_id AND deleted_at IS NULL
ORDER BY skill_name;

-- name: QuerySelectSkillsByTags :many
SELECT * FROM user_skills
WHERE user_id = @user_id AND tags && @tags::TEXT[] AND deleted_at IS NULL;

-- name: QueryInsertSkill :one
INSERT INTO user_skills(user_id, skill_name, proficiency_level, tags)
VALUES (@user_id, @skill_name, @proficiency_level, @tags)
RETURNING *;

-- name: QueryUpdateSkill :exec
UPDATE user_skills
SET
    skill_name        = @skill_name,
    proficiency_level = @proficiency_level,
    tags              = @tags,
    updated_at        = now()
WHERE id = @id AND user_id = @user_id;

-- name: QueryDeleteSkill :exec
UPDATE user_skills
SET deleted_at = now()
WHERE id = @id AND user_id = @user_id;

-- name: QuerySelectAllProjects :many
SELECT * FROM user_projects
WHERE user_id = @user_id AND deleted_at IS NULL
ORDER BY start_date DESC;

-- name: QuerySelectProjectsByTags :many
SELECT * FROM user_projects
WHERE user_id = @user_id AND tags && @tags::TEXT[] AND deleted_at IS NULL;

-- name: QueryInsertProject :one
INSERT INTO user_projects(
    user_id, project_name, description, project_url,
    tags, start_date, end_date, is_academic
) VALUES (
    @user_id, @project_name, @description, @project_url,
    @tags, @start_date, @end_date, @is_academic
) RETURNING *;

-- name: QueryUpdateProject :exec
UPDATE user_projects
SET
    project_name = @project_name,
    description  = @description,
    project_url  = @project_url,
    tags         = @tags,
    start_date   = @start_date,
    end_date     = @end_date,
    is_academic  = @is_academic,
    updated_at   = now()
WHERE id = @id AND user_id = @user_id;

-- name: QueryDeleteProject :exec
UPDATE user_projects
SET deleted_at = now()
WHERE id = @id AND user_id = @user_id;

-- name: QuerySelectAllCertificates :many
SELECT * FROM user_certificates
WHERE user_id = @user_id AND deleted_at IS NULL
ORDER BY issue_date DESC;

-- name: QueryInsertCertificate :one
INSERT INTO user_certificates(
    user_id, certificate_name, issuing_organization, issue_date, credential_url, tags
) VALUES (
    @user_id, @certificate_name, @issuing_organization, @issue_date, @credential_url, @tags
) RETURNING *;

-- name: QueryUpdateCertificate :exec
UPDATE user_certificates
SET
    certificate_name     = @certificate_name,
    issuing_organization = @issuing_organization,
    issue_date           = @issue_date,
    credential_url       = @credential_url,
    tags                 = @tags,
    updated_at           = now()
WHERE id = @id AND user_id = @user_id;

-- name: QueryDeleteCertificate :exec
UPDATE user_certificates
SET deleted_at = now()
WHERE id = @id AND user_id = @user_id;
