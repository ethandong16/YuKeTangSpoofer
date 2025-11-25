package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// (ParseRawCookie removed from this file to avoid duplicate across package.)

func mustParseIntList(s string) []int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			log.Fatalf("invalid integer in list: %q: %v", p, err)
		}
		out = append(out, v)
	}
	return out
}

func RunHeartbeatTool() {
	// Flags for user to configure
	userID := flag.Int64("user", 0, "User ID (u)")
	courseID := flag.Int64("course", 0, "Course ID (c)")
	classroomID := flag.String("classroom", "", "Classroom ID (classroomid)")
	rawCookie := flag.String("cookie", "", "Raw Cookie header (e.g. 'a=1; b=2')")
	videos := flag.String("videos", "", "Comma-separated list of video ids (v)")
	durations := flag.String("durations", "", "Comma-separated list of durations in seconds (aligns with videos). If single value provided, used for all videos")
	intervalSec := flag.Int64("interval", 60, "Heartbeat interval in seconds (how much progress each packet activates)")
	endpoint := flag.String("endpoint", "https://www.yuketang.cn/video-log/heartbeat/", "Heartbeat endpoint URL")
	sleepMs := flag.Int("sleep-ms", 600, "Milliseconds to sleep between requests")
	flag.Parse()

	if *userID == 0 || *courseID == 0 || *classroomID == "" || *videos == "" {
		log.Fatalf("must provide -user, -course, -classroom and -videos flags")
	}

	videoIDs := mustParseIntList(*videos)
	if len(videoIDs) == 0 {
		log.Fatalf("no valid video ids parsed from -videos")
	}
	durList := mustParseIntList(*durations)
	// If user provided a single duration, expand it to all videos.
	if len(durList) == 1 && len(videoIDs) > 1 {
		d := durList[0]
		durList = make([]int64, len(videoIDs))
		for i := range durList {
			durList[i] = d
		}
	}
	// If no durations provided, default each video to interval (i.e., one heartbeat)
	if len(durList) == 0 {
		durList = make([]int64, len(videoIDs))
		for i := range durList {
			durList[i] = *intervalSec
		}
	}
	if len(durList) != len(videoIDs) {
		log.Fatalf("number of durations (%d) does not match number of videos (%d)", len(durList), len(videoIDs))
	}

	cookies := ParseRawCookie(*rawCookie)

	client := &http.Client{Timeout: 15 * time.Second}

	// Iterate over videos
	for idx, vid := range videoIDs {
		totalSec := durList[idx]
		log.Printf("video %d (id=%d) duration=%ds", idx+1, vid, totalSec)

		// Start from cp=0, increment by intervalSec until >= totalSec
		for cp := int64(0); cp < totalSec; cp += *intervalSec {
			// Build heartbeat object according to the example payload
			heart := map[string]interface{}{
				"i":           5,
				"et":          "loadstart",
				"p":           "web",
				"n":           "ali-cdn.xuetangx.com",
				"lob":         "ykt",
				"cp":          cp,
				"fp":          0,
				"tp":          cp,
				"sp":          1,
				"ts":          fmt.Sprintf("%d", time.Now().UnixNano()/1e6),
				"u":           *userID,
				"uip":         "",
				"c":           *courseID,
				"v":           vid,
				"skuid":       13318608,
				"classroomid": *classroomID,
				"cc":          "D8E356C1339D5C51B463AB73BD4C026B",
				"d":           0,
				"pg":          fmt.Sprintf("%d_ndl0", vid),
				"sq":          1,
				"t":           "video",
				"cards_id":    0,
				"slide":       0,
				"v_url":       "",
			}

			// The endpoint expects a JSON payload under "heart_data" array.
			body := map[string]interface{}{"heart_data": []interface{}{heart}}
			b, err := json.Marshal(body)
			if err != nil {
				log.Fatalf("json marshal failed: %v", err)
			}

			// Build GET request with params=<json>
			u, err := url.Parse(*endpoint)
			if err != nil {
				log.Fatalf("invalid endpoint: %v", err)
			}
			q := u.Query()
			q.Set("params", string(b))
			u.RawQuery = q.Encode()

			req, err := http.NewRequest("GET", u.String(), nil)
			if err != nil {
				log.Fatalf("create request failed: %v", err)
			}

			// Add cookies by iterating the slice (user requested a for ... range ... to add cookies)
			for _, c := range cookies {
				req.AddCookie(c)
			}

			// Common headers
			req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; heartbeat-bot/1.0)")
			req.Header.Set("Accept", "application/json, text/plain, */*")

			// Send request
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("request error for vid=%d cp=%d: %v", vid, cp, err)
			} else {
				// Read and discard body
				_, _ = io.ReadAll(resp.Body)
				resp.Body.Close()
				log.Printf("sent heartbeat vid=%d cp=%d -> status=%s", vid, cp, resp.Status)
			}

			time.Sleep(time.Duration(*sleepMs) * time.Millisecond)
		}

		// Optionally send one final packet at the exact end to mark completion
		cp := totalSec
		heart := map[string]interface{}{
			"i":           5,
			"et":          "timeupdate",
			"p":           "web",
			"n":           "ali-cdn.xuetangx.com",
			"lob":         "ykt",
			"cp":          cp,
			"fp":          0,
			"tp":          cp,
			"sp":          1,
			"ts":          fmt.Sprintf("%d", time.Now().UnixNano()/1e6),
			"u":           *userID,
			"uip":         "",
			"c":           *courseID,
			"v":           vid,
			"skuid":       13318608,
			"classroomid": *classroomID,
			"cc":          "D8E356C1339D5C51B463AB73BD4C026B",
			"d":           0,
			"pg":          fmt.Sprintf("%d_ndl0", vid),
			"sq":          1,
			"t":           "video",
			"cards_id":    0,
			"slide":       0,
			"v_url":       "",
		}
		body := map[string]interface{}{"heart_data": []interface{}{heart}}
		b, err := json.Marshal(body)
		if err != nil {
			log.Fatalf("json marshal failed: %v", err)
		}
		u, err := url.Parse(*endpoint)
		if err != nil {
			log.Fatalf("invalid endpoint: %v", err)
		}
		q := u.Query()
		q.Set("params", string(b))
		u.RawQuery = q.Encode()
		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			log.Fatalf("create request failed: %v", err)
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; heartbeat-bot/1.0)")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("final packet error vid=%d: %v", vid, err)
		} else {
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("final heartbeat vid=%d -> status=%s", vid, resp.Status)
		}
	}

	log.Println("done")
}
