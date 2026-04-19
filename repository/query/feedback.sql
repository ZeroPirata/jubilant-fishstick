-- name: QueryInsertFeedback :exec
INSERT INTO resumes_feedback(resume_id, user_id, status, comments)
VALUES (@resume_id, @user_id, @status, @comments);

-- name: QuerySelectFeedbackByResume :one
SELECT * FROM resumes_feedback
WHERE resume_id = @resume_id AND user_id = @user_id;

-- name: QuerySelectGoodFeedbacks :many
SELECT rf.comments
FROM resumes_feedback rf
WHERE
    rf.status IN ('good', 'fair') AND
    rf.comments IS NOT NULL AND
    rf.user_id = @user_id
ORDER BY rf.created_at DESC
LIMIT 5;

-- name: QuerySelectExcellentResumes :many
SELECT gr.content_json
FROM resumes_feedback rf
JOIN generated_resumes gr ON gr.id = rf.resume_id
WHERE
    rf.status = 'excellent' AND
    rf.user_id = @user_id
ORDER BY rf.created_at DESC
LIMIT 3;
