package models

import (
	"Parallels/pd-api-service/common"
	"Parallels/pd-api-service/errors"
)

type CreatePackerVirtualMachineRequest struct {
	Template     string `json:"template"`
	Owner        string `json:"owner"`
	Name         string `json:"name"`
	Memory       string `json:"memory"`
	Cpu          string `json:"cpu"`
	Disk         string `json:"disk"`
	DesiredState string `json:"desiredState"`
}

func (r *CreatePackerVirtualMachineRequest) Validate() error {
	if r.Template == "" {
		return errors.New("Template cannot be empty")
	}

	if r.Name == "" {
		return errors.New("Name cannot be empty")
	}

	if r.Memory == "" {
		common.Logger.Info("Memory is less than 0, setting to 2048")
		r.Memory = "2048"
	}

	if r.Owner == "" {
		return errors.New("Owner cannot be empty")
	}

	if r.Cpu == "" {
		common.Logger.Info("CPU is less than 0, setting to 2")
		r.Cpu = "2"
	}

	if r.Disk == "" {
		common.Logger.Info("Disk is less than 0, setting to 20480")
		r.Disk = "20480"
	}

	if r.DesiredState == "" {
		common.Logger.Info("DesiredState is empty, setting to 'running'")
		r.DesiredState = "running"
	}

	return nil
}