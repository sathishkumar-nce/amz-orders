package repository

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository struct {
	db *pgxpool.Pool
}

const legacySeedAdminHash = "$2a$10$6E55sUsAh.UoUMbI1Dik2e5H6xS8zszERg39zKH1BhA92p3FCyFGy"
const DefaultWelcomePassword = "Welcome999"

var ErrUserNotFound = errors.New("user not found")

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser creates a new user with hashed password
func (r *UserRepository) CreateUser(ctx context.Context, req *models.RegisterRequest, actor string, mustChangePassword bool) (*models.User, error) {
	log.Printf("👤 Creating user (username=%s email=%s)", req.Username, req.Email)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO users (username, password, email, must_change_password, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NOW(), NOW())
		RETURNING id, username, email, must_change_password, COALESCE(created_by, ''), COALESCE(updated_by, ''), created_at, updated_at
	`

	var user models.User
	err = tx.QueryRow(ctx, query, req.Username, string(hashedPassword), req.Email, mustChangePassword, actor, actor).
		Scan(&user.ID, &user.Username, &user.Email, &user.MustChangePassword, &user.CreatedBy, &user.UpdatedBy, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	log.Printf("✅ User created (username=%s user_id=%d)", user.Username, user.ID)

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	log.Printf("🔎 Looking up user by username (username=%s)", username)
	query := `
		SELECT id, username, password, email, must_change_password, COALESCE(created_by, ''), COALESCE(updated_by, ''), created_at, updated_at
		FROM users
		WHERE username = $1
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, username).
		Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.MustChangePassword, &user.CreatedBy, &user.UpdatedBy, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("ℹ️  User not found by username (username=%s)", username)
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	log.Printf("✅ User lookup by username succeeded (username=%s user_id=%d)", user.Username, user.ID)

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	log.Printf("🔎 Looking up user by id (user_id=%d)", id)
	query := `
		SELECT id, username, email, must_change_password, COALESCE(created_by, ''), COALESCE(updated_by, ''), created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := r.db.QueryRow(ctx, query, id).
		Scan(&user.ID, &user.Username, &user.Email, &user.MustChangePassword, &user.CreatedBy, &user.UpdatedBy, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("ℹ️  User not found by id (user_id=%d)", id)
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	log.Printf("✅ User lookup by id succeeded (user_id=%d username=%s)", user.ID, user.Username)

	return &user, nil
}

// VerifyPassword checks if the provided password matches the user's password
func (r *UserRepository) VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// EnsureDefaultAdmin creates the default admin user when missing and repairs
// the known legacy seed password mismatch for existing databases.
func (r *UserRepository) EnsureDefaultAdmin(ctx context.Context, username, password, email string) error {
	log.Printf("🛡️  Ensuring default admin exists (username=%s email=%s)", username, email)
	user, err := r.GetUserByUsername(ctx, username)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			return err
		}

		_, err = r.CreateUser(ctx, &models.RegisterRequest{
			Username: username,
			Password: password,
			Email:    email,
		}, "system", false)
		if err == nil {
			log.Printf("✅ Default admin created (username=%s)", username)
		}
		return err
	}

	if r.VerifyPassword(user.Password, password) == nil {
		log.Printf("✅ Default admin already valid (username=%s user_id=%d)", user.Username, user.ID)
		return nil
	}

	if user.Password != legacySeedAdminHash {
		log.Printf("ℹ️  Default admin exists with non-legacy password; leaving unchanged (username=%s user_id=%d)", user.Username, user.ID)
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		UPDATE users
		SET password = $1, must_change_password = FALSE, updated_at = NOW(), updated_by = $3
		WHERE id = $2
	`, string(hashedPassword), user.ID, username)
	if err == nil {
		log.Printf("✅ Default admin legacy password repaired (username=%s user_id=%d)", user.Username, user.ID)
	}

	return err
}

func (r *UserRepository) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, username, email, must_change_password, COALESCE(created_by, ''), COALESCE(updated_by, ''), created_at, updated_at
		FROM users
		ORDER BY username ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.MustChangePassword, &user.CreatedBy, &user.UpdatedBy, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) AdminCreateUser(ctx context.Context, req *models.AdminCreateUserRequest, actor string) (*models.User, error) {
	return r.CreateUser(ctx, &models.RegisterRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: DefaultWelcomePassword,
	}, actor, true)
}

func (r *UserRepository) DeleteUser(ctx context.Context, userID int64) error {
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if user.Username == "admin" {
		return errors.New("the admin user cannot be deleted")
	}

	tag, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *UserRepository) ChangePassword(ctx context.Context, userID int64, currentPassword, newPassword, actor string) error {
	user, err := r.GetUserByIDWithPassword(ctx, userID)
	if err != nil {
		return err
	}
	if err := r.VerifyPassword(user.Password, currentPassword); err != nil {
		return errors.New("current password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		UPDATE users
		SET password = $1, must_change_password = FALSE, updated_at = NOW(), updated_by = $3
		WHERE id = $2
	`, string(hashedPassword), userID, actor)
	return err
}

func (r *UserRepository) GetUserByIDWithPassword(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `
		SELECT id, username, password, email, must_change_password, COALESCE(created_by, ''), COALESCE(updated_by, ''), created_at, updated_at
		FROM users
		WHERE id = $1
	`, id).Scan(&user.ID, &user.Username, &user.Password, &user.Email, &user.MustChangePassword, &user.CreatedBy, &user.UpdatedBy, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}
