package db

import (
	"golang.org/x/crypto/bcrypt"
)

// WebUserCount returns the number of registered web users.
func (d *Database) WebUserCount() (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM web_users").Scan(&count)
	return count, err
}

// CreateWebUser inserts a new web user with a bcrypt-hashed password.
func (d *Database) CreateWebUser(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err = d.db.Exec("INSERT INTO web_users (username, password_hash) VALUES (?, ?)", username, string(hash))
	return err
}

// GetWebUserPasswordHash returns the password hash for the given username.
func (d *Database) GetWebUserPasswordHash(username string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var hash string
	err := d.db.QueryRow("SELECT password_hash FROM web_users WHERE username = ?", username).Scan(&hash)
	return hash, err
}
