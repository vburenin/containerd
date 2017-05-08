package tasks

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/containerd/containerd/plapi/models"
)

// WaitReader is a Reader for the Wait structure.
type WaitReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *WaitReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewWaitOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	case 404:
		result := NewWaitNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	case 500:
		result := NewWaitInternalServerError()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	default:
		return nil, runtime.NewAPIError("unknown error", response, response.Code())
	}
}

// NewWaitOK creates a WaitOK with default headers values
func NewWaitOK() *WaitOK {
	return &WaitOK{}
}

/*WaitOK handles this case with default header values.

OK
*/
type WaitOK struct {
}

func (o *WaitOK) Error() string {
	return fmt.Sprintf("[PUT /tasks][%d] waitOK ", 200)
}

func (o *WaitOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewWaitNotFound creates a WaitNotFound with default headers values
func NewWaitNotFound() *WaitNotFound {
	return &WaitNotFound{}
}

/*WaitNotFound handles this case with default header values.

not found
*/
type WaitNotFound struct {
	Payload *models.Error
}

func (o *WaitNotFound) Error() string {
	return fmt.Sprintf("[PUT /tasks][%d] waitNotFound  %+v", 404, o.Payload)
}

func (o *WaitNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewWaitInternalServerError creates a WaitInternalServerError with default headers values
func NewWaitInternalServerError() *WaitInternalServerError {
	return &WaitInternalServerError{}
}

/*WaitInternalServerError handles this case with default header values.

Wait of task failed
*/
type WaitInternalServerError struct {
	Payload *models.Error
}

func (o *WaitInternalServerError) Error() string {
	return fmt.Sprintf("[PUT /tasks][%d] waitInternalServerError  %+v", 500, o.Payload)
}

func (o *WaitInternalServerError) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
