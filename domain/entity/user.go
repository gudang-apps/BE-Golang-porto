package entity

type User struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

func (u User) HasRole(role string) bool {
	for _, current := range u.Roles {
		if current == role {
			return true
		}
	}
	return false
}
