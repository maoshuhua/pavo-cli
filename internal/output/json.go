package output

import (
	"encoding/json"
	"fmt"
	"io"
)

func WriteJSON(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(writer, string(data))
	return err
}
