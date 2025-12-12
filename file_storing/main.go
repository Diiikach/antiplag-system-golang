package main
import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"log"
)
func main() {
	os.MkdirAll("/files", 0755)
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		r.ParseMultipartForm(20 << 20)
		file, handler, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		fpath := filepath.Join("/files", handler.Filename)
		out, err := os.Create(fpath)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		defer out.Close()
		io.Copy(out, file)
		w.Write([]byte(`{"filename":"` + handler.Filename + `"}`))
	})
	http.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		fname := filepath.Base(r.URL.Path)
		fpath := filepath.Join("/files", fname)
		if _, err := os.Stat(fpath); os.IsNotExist(err) {
			w.WriteHeader(404)
			return
		}
		http.ServeFile(w, r, fpath)
	})
	log.Println("file_storing service running at :8001")
	http.ListenAndServe(":8001", nil)
}
