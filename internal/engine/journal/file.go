package journal

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

func Open(path string) (Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &jsonReader{
		closer: f,
		dec:    json.NewDecoder(f),
		epoch:  time.Now(),
	}, nil
}

type jsonReader struct {
	closer io.Closer
	dec    *json.Decoder
	epoch  time.Time
	start  time.Time
}

func (r *jsonReader) ReadEntry() (*Entry, bool) {
	var e Entry
	if err := r.dec.Decode(&e); err != nil {
		return nil, false
	}
	if r.start.IsZero() {
		r.start = e.TS
	} else {
		target := r.epoch.Add(e.TS.Sub(r.start))
		duration := time.Until(target)
		if duration > 0 {
			time.Sleep(duration)
		}
	}
	return &e, true
}

func (r jsonReader) Close() error {
	return r.closer.Close()
}
