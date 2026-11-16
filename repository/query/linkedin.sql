-- name: LinkedInListSearches :many
SELECT * FROM user_linkedin_searches
WHERE user_id = @user_id
ORDER BY created_at;

-- name: LinkedInCountSearches :one
SELECT COUNT(*) FROM user_linkedin_searches
WHERE user_id = @user_id;

-- name: LinkedInInsertSearch :one
INSERT INTO user_linkedin_searches(
    user_id, name, keywords, geo_id, geo_label, posted_within, workplace, experience,
    job_types, easy_apply, few_applicants, in_network, sort, auto
) VALUES (
    @user_id, @name, @keywords, @geo_id, @geo_label, @posted_within, @workplace, @experience,
    @job_types, @easy_apply, @few_applicants, @in_network, @sort, @auto
)
RETURNING *;

-- name: LinkedInUpdateSearch :one
UPDATE user_linkedin_searches SET
    name = @name, keywords = @keywords, geo_id = @geo_id, geo_label = @geo_label,
    posted_within = @posted_within, workplace = @workplace, experience = @experience,
    job_types = @job_types, easy_apply = @easy_apply, few_applicants = @few_applicants,
    in_network = @in_network, sort = @sort, updated_at = now()
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: LinkedInDeleteSearch :execrows
DELETE FROM user_linkedin_searches
WHERE id = @id AND user_id = @user_id;

-- name: LinkedInDeleteAutoSearches :exec
DELETE FROM user_linkedin_searches
WHERE user_id = @user_id AND auto;
