package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
)

func main() {
	fileStoringURL := "http://file_storing:8001"
	fileAnalysisURL := "http://file_analysis:8002"
	http.HandleFunc("/api/submit", func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(20 << 20)
		file, handler, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		// Upload file to file_storing
		buf := &bytes.Buffer{}
		mw := multipart.NewWriter(buf)
		fw, _ := mw.CreateFormFile("file", handler.Filename)
		io.Copy(fw, file)
		mw.Close()
		req, _ := http.NewRequest("POST", fileStoringURL+"/upload", buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			w.WriteHeader(502)
			return
		}
		// Call file_analysis
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		ana := map[string]string{"file_name": handler.Filename, "sender": r.FormValue("sender"), "work_id": r.FormValue("work_id")}
		body, _ := json.Marshal(ana)
		resp2, err := http.Post(fileAnalysisURL+"/analyze", "application/json", bytes.NewReader(body))
		if err != nil {
			w.WriteHeader(502)
			return
		}
		io.Copy(w, resp2.Body)
		resp2.Body.Close()
	})

	http.HandleFunc("/api/works/", func(w http.ResponseWriter, r *http.Request) {
		// /api/works/{work_id}/reports
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		workid := parts[len(parts)-2]
		url := fileAnalysisURL + "/reports/" + workid
		resp, err := http.Get(url)
		if err != nil {
			w.WriteHeader(502)
			return
		}
		if resp.StatusCode != 200 {
			w.WriteHeader(404)
			return
		}
		io.Copy(w, resp.Body)
		resp.Body.Close()
	})
	log.Println("gateway running at :8000")
	http.ListenAndServe(":8000", nil)
}
