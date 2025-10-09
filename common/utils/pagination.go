package utils

func Pagination[T any](list []T, pageSize int, handler func(page int, slice []T) error) error {
	if pageSize <= 0 {
		return nil
	}

	total := len(list)
	if total == 0 {
		return nil
	}

	pageCount := total / pageSize
	if total%pageSize != 0 {
		pageCount++
	}

	for i := 0; i < pageCount; i++ {
		start := i * pageSize
		end := start + pageSize
		if end > total {
			end = total
		}

		if err := handler(i, list[start:end]); err != nil {
			return err
		}
	}

	return nil
}
