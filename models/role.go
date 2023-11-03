package models

import "Parallels/pd-api-service/errors"

type RoleRequest struct {
	Name string `json:"name"`
}

func (r *RoleRequest) Validate() error {
	if r.Name == "" {
		return errors.NewWithCode("Role name cannot be empty", 400)
	}

	return nil
}

type RoleResponse struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}
