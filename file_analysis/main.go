package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"log"
	"crypto/sha1"
	"math"
)

type AnalyzeRequest struct {
	FileName string `json:"file_name"`
	Sender string    `json:"sender"`
	WorkID string    `json:"work_id"`
}
type Report struct {
	FileName   string  `json:"file_name"`
	Sender     string  `json:"sender"`
	WorkID     string  `json:"work_id"`
	Plagiarised bool    `json:"plagiarised"`
	Timestamp  int64   `json:"timestamp"`
	Similarity float32 `json:"similarity"`
}

func textToVector(text string) []float32 {
	h := sha1.Sum([]byte(text)) // demo: hash вместо реального embedding
	var v []float32
	for i := 0; i < 8; i++ {
		v = append(v, float32(h[i])/255)
	}
	return v
}

func cosineSim(a, b []float32) float32 {
	var dot, mA, mB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		mA += float64(a[i] * a[i])
		mB += float64(b[i] * b[i])
	}
	if mA == 0 || mB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(mA) * math.Sqrt(mB)))
}

func main() {
	repDir := "/files/reports"
	os.MkdirAll(repDir, 0755)
	qdrantURL := "http://qdrant:6333"
	coll := "works"

	http.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req AnalyzeRequest
		json.NewDecoder(r.Body).Decode(&req)
		contentPath := filepath.Join("/files", req.FileName)
		b, err := ioutil.ReadFile(contentPath)
		if err != nil {
			w.WriteHeader(422)
			return
		}
		vec := textToVector(string(b))
		// upsert в qdrant
		payload := map[string]interface{}{
			"file_name": req.FileName,
			"sender": req.Sender,
			"work_id": req.WorkID,
		}
		id := fmt.Sprintf("%s_%s", req.Sender, req.FileName)
		// хак: делаем upsert через http
		doc := map[string]interface{}{
			"points": []map[string]interface{}{
				{
					"id": id,
					"vector": vec,
					"payload": payload,
				},
			},
		}
		jsonDoc, _ := json.Marshal(doc)
		resp, err := http.Post(qdrantURL+"/collections/"+coll+"/points?wait=true", "application/json", bytes.NewReader(jsonDoc))
		if err != nil || resp.StatusCode >= 400 {
			w.WriteHeader(502)
			return
		}
		resp.Body.Close()
		// поиск векторов других пользователей (по work_id)
		searchReq := map[string]interface{}{
			"vector": vec,
			"top": 1,
			"filter": map[string]interface{}{
				"must": []map[string]interface{}{
					{"key": "work_id", "match": map[string]interface{}{"value": req.WorkID}},
					{"key": "sender", "match": map[string]interface{}{"value": map[string]interface{}{"$ne": req.Sender}}},
				},
			},
		}
		jsonSearch, _ := json.Marshal(searchReq)
		resp2, err := http.Post(qdrantURL+"/collections/"+coll+"/points/search", "application/json", bytes.NewReader(jsonSearch))
		var sim float32
		plag := false
		if err == nil && resp2.StatusCode == 200 {
			var result struct {
				Result []struct{ Score float32 `json:"score"` }
			}
			json.NewDecoder(resp2.Body).Decode(&result)
			resp2.Body.Close()
			if len(result.Result) > 0 {
				sim = result.Result[0].Score
				if sim > 0.95 {
					plag = true
				}
			}
		}
		report := Report{FileName: req.FileName, Sender: req.Sender, WorkID: req.WorkID, Plagiarised: plag, Timestamp: time.Now().Unix(), Similarity: sim}
		out, _ := os.Create(filepath.Join(repDir, req.Sender+"_"+req.WorkID+".json"))
		json.NewEncoder(out).Encode(report)
		out.Close()
		json.NewEncoder(w).Encode(report)
	})

	http.HandleFunc("/reports/", func(w http.ResponseWriter, r *http.Request) {
		workid := filepath.Base(r.URL.Path)
		var results []Report
		reports, _ := ioutil.ReadDir(repDir)
		for _, rf := range reports {
			f, _ := os.Open(filepath.Join(repDir, rf.Name()))
			var rep Report
			json.NewDecoder(f).Decode(&rep)
			f.Close()
			if rep.WorkID == workid {
				results = append(results, rep)
			}
		}
		if len(results) == 0 {
			w.WriteHeader(404)
			return
		}
		json.NewEncoder(w).Encode(results)
	})
	log.Println("file_analysis service with vector search running at :8002")
	http.ListenAndServe(":8002", nil)
}
