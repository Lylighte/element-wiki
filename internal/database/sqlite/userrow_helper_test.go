package sqlite

import "element-wiki/internal/model"

func userRow(id string) model.User {
	return model.User{ID: id, Issuer: "i", Subject: id,
		DisplayName: id, Role: "viewer", Status: "active", CreatedAt: 1}
}
