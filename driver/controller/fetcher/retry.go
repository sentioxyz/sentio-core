package fetcher

type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return e.Err.Error()
}

func Permanent(err error) error {
	return &PermanentError{
		Err: err,
	}
}
