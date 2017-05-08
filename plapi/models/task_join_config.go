package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// TaskJoinConfig task join config
// swagger:model TaskJoinConfig
type TaskJoinConfig struct {

	// args
	Args []string `json:"args"`

	// attach
	Attach bool `json:"attach,omitempty"`

	// env
	Env []string `json:"env"`

	// handle
	// Required: true
	Handle interface{} `json:"handle"`

	// id
	ID string `json:"id,omitempty"`

	// open stdin
	OpenStdin bool `json:"openStdin,omitempty"`

	// path
	// Required: true
	Path string `json:"path"`

	// stop signal
	StopSignal string `json:"stopSignal,omitempty"`

	// tty
	Tty bool `json:"tty,omitempty"`

	// user
	User string `json:"user,omitempty"`

	// working dir
	WorkingDir string `json:"workingDir,omitempty"`
}

// Validate validates this task join config
func (m *TaskJoinConfig) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateArgs(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if err := m.validateEnv(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if err := m.validateHandle(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if err := m.validatePath(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *TaskJoinConfig) validateArgs(formats strfmt.Registry) error {

	if swag.IsZero(m.Args) { // not required
		return nil
	}

	return nil
}

func (m *TaskJoinConfig) validateEnv(formats strfmt.Registry) error {

	if swag.IsZero(m.Env) { // not required
		return nil
	}

	return nil
}

func (m *TaskJoinConfig) validateHandle(formats strfmt.Registry) error {

	return nil
}

func (m *TaskJoinConfig) validatePath(formats strfmt.Registry) error {

	if err := validate.RequiredString("path", "body", string(m.Path)); err != nil {
		return err
	}

	return nil
}
