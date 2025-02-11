// Package binding deserializes data from HTTP requests into a struct
// ready for your application to use (without reflection). It also
// facilitates data validation and error handling.
package binding

import (
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type requestBinder func(req *http.Request, userStruct FieldMapper) Errors

// Bind takes data out of the request and deserializes into a struct according
// to the Content-Type of the request. If no Content-Type is specified, there
// better be data in the query string, otherwise an error will be produced.
//
// A non-nil return value may be an Errors value.
func Bind(req *http.Request, userStruct FieldMapper) error {
	var errs Errors

	contentType := req.Header.Get("Content-Type")

	if strings.Contains(contentType, "form-urlencoded") {
		return Form(req, userStruct)
	}

	if strings.Contains(contentType, "multipart/form-data") {
		return MultipartForm(req, userStruct)
	}

	if strings.Contains(contentType, "json") {
		return Json(req, userStruct)
	}

	if req.Method == http.MethodGet || req.Method == http.MethodHead || (contentType == "" && len(req.URL.Query()) > 0) {
		return URL(req, userStruct)
	}

	if contentType == "" {
		errs.Add([]string{}, ContentTypeError, "Empty Content-Type")
		errs = validate(errs, req, userStruct)
	} else {
		errs.Add([]string{}, ContentTypeError, "Unsupported Content-Type")
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// Form deserializes form data out of the request into a struct you provide.
// This function invokes data validation after deserialization.
func Form(req *http.Request, userStruct FieldMapper) error {
	err := formBinder(req, userStruct)
	if len(err) > 0 {
		return err
	}
	return nil
}

var formBinder requestBinder = defaultFormBinder

func defaultFormBinder(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	parseErr := req.ParseForm()
	if parseErr != nil {
		errs.Add([]string{}, DeserializationError, parseErr.Error())
		return errs
	}

	return bindForm(req, userStruct, req.Form, nil)
}

// URL reads data out of the query string into a struct you provide.
// This function invokes data validation after deserialization.
func URL(req *http.Request, userStruct FieldMapper) error {
	err := urlBinder(req, userStruct)
	if len(err) > 0 {
		return err
	}
	return nil

}

var urlBinder requestBinder = defaultURLBinder

func defaultURLBinder(req *http.Request, userStruct FieldMapper) Errors {
	return bindForm(req, userStruct, req.URL.Query(), nil)
}

// MultipartForm reads a multipart form request and deserializes its data and
// files into a struct you provide. Files should be deserialized into
// *multipart.FileHeader fields.
func MultipartForm(req *http.Request, userStruct FieldMapper) error {
	err := multipartFormBinder(req, userStruct)
	if len(err) > 0 {
		return err
	}

	return nil
}

var multipartFormBinder requestBinder = defaultMultipartFormBinder

func defaultMultipartFormBinder(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	if req.MultipartForm == nil {
		multipartReader, err := req.MultipartReader()
		if err != nil {
			errs.Add([]string{}, DeserializationError, err.Error())
			return errs
		}

		form, parseErr := multipartReader.ReadForm(MaxMemory)
		if parseErr != nil {
			errs.Add([]string{}, DeserializationError, parseErr.Error())
			return errs
		}

		req.MultipartForm = form
	}

	return bindForm(req, userStruct, req.MultipartForm.Value, req.MultipartForm.File)
}

// Json deserializes a JSON request body into a struct you specify
// using the standard encoding/json package (which uses reflection).
// This function invokes data validation after deserialization.
func Json(req *http.Request, userStruct FieldMapper) error {
	err := jsonBinder(req, userStruct)
	if len(err) > 0 {
		return err
	}

	return nil
}

var jsonBinder requestBinder = defaultJsonBinder

func defaultJsonBinder(req *http.Request, userStruct FieldMapper) Errors {
	var errs Errors

	if req.Body != nil {
		defer req.Body.Close()
		err := json.NewDecoder(req.Body).Decode(userStruct)
		if err != nil && err != io.EOF {
			errs.Add([]string{}, DeserializationError, err.Error())
			return errs
		}
	} else {
		errs.Add([]string{}, DeserializationError, "Empty request body")
		return errs
	}

	errs = validate(errs, req, userStruct)
	if len(errs) > 0 {
		return errs
	}

	return nil
}

// Validate ensures that all conditions have been met on every field in the
// populated struct. Validation should occur after the request has been
// deserialized into the struct.
func Validate(req *http.Request, userStruct FieldMapper) error {
	err := validate(Errors{}, req, userStruct)
	if len(err) > 0 {
		return err
	}

	return nil
}

func validate(errs Errors, req *http.Request, userStruct FieldMapper) Errors {
	fm := userStruct.FieldMap(req)

	for fieldPointer, fieldNameOrSpec := range fm {
		fieldSpec, err := fieldSpecification(fieldNameOrSpec)
		if err != nil {
			continue
		}

		addRequiredError := func() {
			errorMsg := "Required"
			if len(fieldSpec.ErrorMessage) > 0 {
				errorMsg = fieldSpec.ErrorMessage
			}

			errs.Add([]string{fieldSpec.Form}, RequiredError, errorMsg)
		}
		if fieldSpec.Required {
			switch t := fieldPointer.(type) {
			case *uint8:
				if *t == 0 {
					addRequiredError()
				}
			case **uint8:
				if *t == nil {
					addRequiredError()
				}
			case *[]uint8:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *uint16:
				if *t == 0 {
					addRequiredError()
				}
			case **uint16:
				if *t == nil {
					addRequiredError()
				}
			case *[]uint16:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *uint32:
				if *t == 0 {
					addRequiredError()
				}
			case **uint32:
				if *t == nil {
					addRequiredError()
				}
			case *[]uint32:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *uint64:
				if *t == 0 {
					addRequiredError()
				}
			case **uint64:
				if *t == nil {
					addRequiredError()
				}
			case *[]uint64:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *int8:
				if *t == 0 {
					addRequiredError()
				}
			case **int8:
				if *t == nil {
					addRequiredError()
				}
			case *[]int8:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *int16:
				if *t == 0 {
					addRequiredError()
				}
			case **int16:
				if *t == nil {
					addRequiredError()
				}
			case *[]int16:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *int32:
				if *t == 0 {
					addRequiredError()
				}
			case **int32:
				if *t == nil {
					addRequiredError()
				}
			case *[]int32:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *int64:
				if *t == 0 {
					addRequiredError()
				}
			case **int64:
				if *t == nil {
					addRequiredError()
				}
			case *[]int64:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *float32:
				if *t == 0 {
					addRequiredError()
				}
			case **float32:
				if *t == nil {
					addRequiredError()
				}
			case *[]float32:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *float64:
				if *t == 0 {
					addRequiredError()
				}
			case **float64:
				if *t == nil {
					addRequiredError()
				}
			case *[]float64:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *uint:
				if *t == 0 {
					addRequiredError()
				}
			case **uint:
				if *t == nil {
					addRequiredError()
				}
			case *[]uint:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *int:
				if *t == 0 {
					addRequiredError()
				}
			case **int:
				if *t == nil {
					addRequiredError()
				}
			case *[]int:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *bool:
				if *t == false {
					addRequiredError()
				}
			case **bool:
				if *t == nil {
					addRequiredError()
				}
			case *[]bool:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *string:
				if *t == "" {
					addRequiredError()
				}
			case **string:
				if *t == nil {
					addRequiredError()
				}
			case *[]string:
				if len(*t) == 0 {
					addRequiredError()
				}
			case *time.Time:
				if t.IsZero() {
					addRequiredError()
				}
			case **time.Time:
				if *t == nil {
					addRequiredError()
				}
			case *[]time.Time:
				if len(*t) == 0 {
					addRequiredError()
				}
			case **multipart.FileHeader:
				if *t == nil {
					addRequiredError()
				}
			}
		}
	}

	if validator, ok := userStruct.(Validator); ok {
		err := validator.Validate(req)
		if err != nil {
			switch e := err.(type) {
			case Error:
				errs = append(errs, e)
			case Errors:
				errs = append(errs, e...)
			default:
				errs.Add([]string{}, "", e.Error())
			}
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func bindForm(req *http.Request, userStruct FieldMapper, formData map[string][]string,
	formFile map[string][]*multipart.FileHeader) Errors {

	var errs Errors

	fm := userStruct.FieldMap(req)

	for fieldPointer, fieldNameOrSpec := range fm {
		fieldSpec, err := fieldSpecification(fieldNameOrSpec)
		if err != nil {
			continue
		}

		strs := formData[fieldSpec.Form]
		_, isFile := fieldPointer.(**multipart.FileHeader)
		_, isFileSlice := fieldPointer.(*[]*multipart.FileHeader)

		if !isFile && !isFileSlice {
			if fieldSpec.Binder != nil {
				err := fieldSpec.Binder(fieldSpec.Form, strs)
				if err != nil {
					switch e := err.(type) {
					case Error:
						errs = append(errs, e)
					case Errors:
						errs = append(errs, e...)
					default:
						errs.Add([]string{fieldSpec.Form}, "", e.Error())
					}
				}
				continue
			}

			if binder, ok := fieldPointer.(Binder); ok {
				err := binder.Bind(fieldSpec.Form, strs)
				if err != nil {
					switch e := err.(type) {
					case Error:
						errs = append(errs, e)
					case Errors:
						errs = append(errs, e...)
					default:
						errs.Add([]string{fieldSpec.Form}, "", e.Error())
					}
				}
				continue
			}

			if len(strs) == 0 {
				continue
			}
		}

		errorHandler := func(err error) {
			if err != nil {
				errs.Add([]string{fieldSpec.Form}, TypeError, err.Error())
			}
		}

		switch t := fieldPointer.(type) {
		case *uint8:
			val, err := strconv.ParseUint(strs[0], 10, 8)
			errorHandler(err)
			*t = uint8(val)
		case **uint8:
			parsed, err := strconv.ParseUint(strs[0], 10, 8)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := uint8(parsed)
			*t = &val
		case *[]uint8:
			for _, str := range strs {
				val, err := strconv.ParseUint(str, 10, 8)
				errorHandler(err)
				*t = append(*t, uint8(val))
			}
		case *uint16:
			val, err := strconv.ParseUint(strs[0], 10, 16)
			errorHandler(err)
			*t = uint16(val)
		case **uint16:
			parsed, err := strconv.ParseUint(strs[0], 10, 16)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := uint16(parsed)
			*t = &val
		case *[]uint16:
			for _, str := range strs {
				val, err := strconv.ParseUint(str, 10, 16)
				errorHandler(err)
				*t = append(*t, uint16(val))
			}
		case *uint32:
			val, err := strconv.ParseUint(strs[0], 10, 32)
			errorHandler(err)
			*t = uint32(val)
		case **uint32:
			parsed, err := strconv.ParseUint(strs[0], 10, 32)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := uint32(parsed)
			*t = &val
		case *[]uint32:
			for _, str := range strs {
				val, err := strconv.ParseUint(str, 10, 32)
				errorHandler(err)
				*t = append(*t, uint32(val))
			}
		case *uint64:
			val, err := strconv.ParseUint(strs[0], 10, 64)
			errorHandler(err)
			*t = val
		case **uint64:
			parsed, err := strconv.ParseUint(strs[0], 10, 64)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := uint64(parsed)
			*t = &val
		case *[]uint64:
			for _, str := range strs {
				val, err := strconv.ParseUint(str, 10, 64)
				errorHandler(err)
				*t = append(*t, uint64(val))
			}
		case *int8:
			val, err := strconv.ParseInt(strs[0], 10, 8)
			errorHandler(err)
			*t = int8(val)
		case **int8:
			parsed, err := strconv.ParseInt(strs[0], 10, 8)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := int8(parsed)
			*t = &val
		case *[]int8:
			for _, str := range strs {
				val, err := strconv.ParseInt(str, 10, 8)
				errorHandler(err)
				*t = append(*t, int8(val))
			}
		case *int16:
			val, err := strconv.ParseInt(strs[0], 10, 16)
			errorHandler(err)
			*t = int16(val)
		case **int16:
			parsed, err := strconv.ParseInt(strs[0], 10, 16)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := int16(parsed)
			*t = &val
		case *[]int16:
			for _, str := range strs {
				val, err := strconv.ParseInt(str, 10, 16)
				errorHandler(err)
				*t = append(*t, int16(val))
			}
		case *int32:
			val, err := strconv.ParseInt(strs[0], 10, 32)
			errorHandler(err)
			*t = int32(val)
		case **int32:
			parsed, err := strconv.ParseInt(strs[0], 10, 32)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := int32(parsed)
			*t = &val
		case *[]int32:
			for _, str := range strs {
				val, err := strconv.ParseInt(str, 10, 32)
				errorHandler(err)
				*t = append(*t, int32(val))
			}
		case *int64:
			val, err := strconv.ParseInt(strs[0], 10, 64)
			errorHandler(err)
			*t = val
		case **int64:
			parsed, err := strconv.ParseInt(strs[0], 10, 64)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := int64(parsed)
			*t = &val
		case *[]int64:
			for _, str := range strs {
				val, err := strconv.ParseInt(str, 10, 64)
				errorHandler(err)
				*t = append(*t, int64(val))
			}
		case *float32:
			val, err := strconv.ParseFloat(strs[0], 32)
			errorHandler(err)
			*t = float32(val)
		case **float32:
			parsed, err := strconv.ParseFloat(strs[0], 32)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := float32(parsed)
			*t = &val
		case *[]float32:
			for _, str := range strs {
				val, err := strconv.ParseFloat(str, 32)
				errorHandler(err)
				*t = append(*t, float32(val))
			}
		case *float64:
			val, err := strconv.ParseFloat(strs[0], 64)
			errorHandler(err)
			*t = val
		case **float64:
			parsed, err := strconv.ParseFloat(strs[0], 64)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := float64(parsed)
			*t = &val
		case *[]float64:
			for _, str := range strs {
				val, err := strconv.ParseFloat(str, 64)
				errorHandler(err)
				*t = append(*t, val)
			}
		case *uint:
			val, err := strconv.ParseUint(strs[0], 10, 0)
			errorHandler(err)
			*t = uint(val)
		case **uint:
			parsed, err := strconv.ParseUint(strs[0], 10, 0)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := uint(parsed)
			*t = &val
		case *[]uint:
			for _, str := range strs {
				val, err := strconv.ParseUint(str, 10, 0)
				errorHandler(err)
				*t = append(*t, uint(val))
			}
		case *int:
			val, err := strconv.ParseInt(strs[0], 10, 0)
			errorHandler(err)
			*t = int(val)
		case **int:
			parsed, err := strconv.ParseInt(strs[0], 10, 0)
			if err != nil {
				errorHandler(err)
				continue
			}
			val := int(parsed)
			*t = &val
		case *[]int:
			for _, str := range strs {
				val, err := strconv.ParseInt(str, 10, 0)
				errorHandler(err)
				*t = append(*t, int(val))
			}
		case *bool:
			val, err := strconv.ParseBool(strs[0])
			errorHandler(err)
			*t = val
		case **bool:
			val, err := strconv.ParseBool(strs[0])
			if err != nil {
				errorHandler(err)
				continue
			}
			*t = &val
		case *[]bool:
			for _, str := range strs {
				val, err := strconv.ParseBool(str)
				errorHandler(err)
				*t = append(*t, val)
			}
		case *string:
			*t = strs[0]
		case **string:
			s := strs[0]
			*t = &s
		case *[]string:
			*t = strs
		case *time.Time:
			timeFormat := TimeFormat
			if fieldSpec.TimeFormat != "" {
				timeFormat = fieldSpec.TimeFormat
			}
			val, err := time.Parse(timeFormat, strs[0])
			errorHandler(err)
			*t = val
		case **time.Time:
			timeFormat := TimeFormat
			if fieldSpec.TimeFormat != "" {
				timeFormat = fieldSpec.TimeFormat
			}
			val, err := time.Parse(timeFormat, strs[0])
			if err != nil {
				errorHandler(err)
				continue
			}
			*t = &val
		case *[]time.Time:
			timeFormat := TimeFormat
			if fieldSpec.TimeFormat != "" {
				timeFormat = fieldSpec.TimeFormat
			}
			for _, str := range strs {
				val, err := time.Parse(timeFormat, str)
				errorHandler(err)
				*t = append(*t, val)
			}
		case **multipart.FileHeader:
			if files, ok := formFile[fieldSpec.Form]; ok {
				*t = files[0]
			}

		case *[]*multipart.FileHeader:
			if files, ok := formFile[fieldSpec.Form]; ok {
				for _, file := range files {
					*t = append(*t, file)
				}
			}
		default:
			errorHandler(errors.New("Field type is unsupported by the application"))
		}
	}

	return validate(errs, req, userStruct)
}

func fieldSpecification(fieldNameOrSpec interface{}) (Field, error) {
	var f Field

	switch vt := fieldNameOrSpec.(type) {
	case Field:
		f = vt
	case string:
		f.Form = vt
	default:
		return f, errors.New("invalid field specification")
	}

	return f, nil
}

type (
	// Only types that are FieldMappers can have request data deserialized into them.
	FieldMapper interface {
		// FieldMap returns a map of pointers into which the values will
		// be deserialized to field names from the request's form body.
		FieldMap(*http.Request) FieldMap
	}

	// FieldMap is a map of pointers to struct fields -> field names from the request.
	// The values could also be Field structs to specify metadata about the field.
	FieldMap map[interface{}]interface{}

	// Field describes the properties of a struct field.
	Field struct {
		// Target is the struct field to deserialize into.
		//Target interface{}

		// Form is the form field name to bind from
		Form string

		// Required indicates whether the field is required. A required
		// field that deserializes into the zero value for that type
		// will generate an error.
		Required bool

		// TimeFormat specifies the time format for time.Time fields.
		TimeFormat string

		// Binder is a function that converts the incoming request value(s)
		// to the field type; in other words, this field is populated
		// by executing this function. Useful when the custom type doesn't
		// implement the Binder interface.
		Binder func(string, []string) error

		// ErrorMessage allows the error the to be customized.
		ErrorMessage string
	}

	// Binder is an interface which can deserialize itself from a slice of string
	// coming from the request. Implement this interface so the type can be
	// populated from form data in HTTP requests.
	Binder interface {
		// Bind populates the type with data in []string which comes from the
		// HTTP request. The first argument is the field name.
		Bind(string, []string) error
	}

	// Validator can be implemented by your type to handle some
	// rudimentary request validation separately from your
	// application logic.
	Validator interface {
		// Validate validates that the request is OK. It is recommended
		// that validation be limited to checking values for syntax and
		// semantics, enough to know that you can make sense of the request
		// in your application. For example, you might verify that a credit
		// card number matches a valid pattern, but you probably wouldn't
		// perform an actual credit card authorization here.
		Validate(*http.Request) error
	}
)

var (
	// Maximum amount of memory to use when parsing a multipart form.
	// Set this to whatever value you prefer; default is 10 MB.
	MaxMemory = int64(1024 * 1024 * 10)

	// If no TimeFormat is specified for a time.Time field, this
	// format will be used by default when parsing.
	TimeFormat = time.RFC3339
)

const (
	jsonContentType           = "application/json; charset=utf-8"
	StatusUnprocessableEntity = 422
)
