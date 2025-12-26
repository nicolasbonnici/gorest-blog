package hooks

import (
	"context"
	"log"

	"github.com/nicolasbonnici/gorest/hooks"
	"golang.org/x/crypto/bcrypt"
)

type UserHooks struct{}

type User struct {
	Id        string  `json:"id,omitempty" db:"id"`
	Firstname string  `json:"firstname" db:"firstname"`
	Lastname  string  `json:"lastname" db:"lastname"`
	Email     string  `json:"email" db:"email"`
	Password  *string `json:"password,omitempty" db:"password"`
}

func (h *UserHooks) StateProcessor(ctx context.Context, operation hooks.Operation, id any, user *User) error {
	if operation == hooks.OperationCreate || operation == hooks.OperationUpdate {
		if user.Password != nil && *user.Password != "" {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*user.Password), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("Error hashing password: %v", err)
				return err
			}
			hashed := string(hashedPassword)
			user.Password = &hashed
			log.Printf("StateProcessor: Password hashed for user %s", user.Email)
		}
	}
	return nil
}

func (h *UserHooks) BeforeQuery(ctx context.Context, operation hooks.Operation, query string, args []any) (string, []any, error) {
	return query, args, nil
}

func (h *UserHooks) AfterQuery(ctx context.Context, operation hooks.Operation, query string, args []any, result any, err error) error {
	return nil
}

func (h *UserHooks) OverrideQuery(ctx context.Context, operation hooks.Operation, id any, model *User) (query string, args []any, skip bool) {
	return "", nil, false
}

func (h *UserHooks) SerializeOne(ctx context.Context, operation hooks.Operation, user *User) error {
	user.Password = nil
	return nil
}

func (h *UserHooks) SerializeMany(ctx context.Context, operation hooks.Operation, users *[]User) error {
	for i := range *users {
		(*users)[i].Password = nil
	}
	return nil
}
