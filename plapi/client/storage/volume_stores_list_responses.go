package storage

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/containerd/containerd/plapi/models"
)

// VolumeStoresListReader is a Reader for the VolumeStoresList structure.
type VolumeStoresListReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *VolumeStoresListReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewVolumeStoresListOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	case 404:
		result := NewVolumeStoresListNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	case 500:
		result := NewVolumeStoresListInternalServerError()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	default:
		return nil, runtime.NewAPIError("unknown error", response, response.Code())
	}
}

// NewVolumeStoresListOK creates a VolumeStoresListOK with default headers values
func NewVolumeStoresListOK() *VolumeStoresListOK {
	return &VolumeStoresListOK{}
}

/*VolumeStoresListOK handles this case with default header values.

OK
*/
type VolumeStoresListOK struct {
	Payload *models.VolumeStoresListResponse
}

func (o *VolumeStoresListOK) Error() string {
	return fmt.Sprintf("[GET /storage/volumestores][%d] volumeStoresListOK  %+v", 200, o.Payload)
}

func (o *VolumeStoresListOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.VolumeStoresListResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewVolumeStoresListNotFound creates a VolumeStoresListNotFound with default headers values
func NewVolumeStoresListNotFound() *VolumeStoresListNotFound {
	return &VolumeStoresListNotFound{}
}

/*VolumeStoresListNotFound handles this case with default header values.

Not found
*/
type VolumeStoresListNotFound struct {
	Payload *models.Error
}

func (o *VolumeStoresListNotFound) Error() string {
	return fmt.Sprintf("[GET /storage/volumestores][%d] volumeStoresListNotFound  %+v", 404, o.Payload)
}

func (o *VolumeStoresListNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewVolumeStoresListInternalServerError creates a VolumeStoresListInternalServerError with default headers values
func NewVolumeStoresListInternalServerError() *VolumeStoresListInternalServerError {
	return &VolumeStoresListInternalServerError{}
}

/*VolumeStoresListInternalServerError handles this case with default header values.

error
*/
type VolumeStoresListInternalServerError struct {
	Payload *models.Error
}

func (o *VolumeStoresListInternalServerError) Error() string {
	return fmt.Sprintf("[GET /storage/volumestores][%d] volumeStoresListInternalServerError  %+v", 500, o.Payload)
}

func (o *VolumeStoresListInternalServerError) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
