package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// CourseEntry represents one selectable course row from the course list
type CourseEntry struct {
	CourseID    string
	ClassroomID string
	Name        string
}

func main() {
	//var inputStrings string
	var courseListJson string
	var parsedCourseList map[string]interface{}
	/*
		fmt.Printf("Enter your YKT Cookie:\n")
		Reader := bufio.NewReader(os.Stdin)
		inputStrings, _ = Reader.ReadString('\n')
		Cookie := ParseRawCookie(inputStrings)
		courseListJson = GetCourseList(Cookie)
	*/
	data, err := os.ReadFile("courseListJson.txt")
	if err != nil {
		panic(err)
	}

	courseListJson = string(data)
	parsedCourseList = UnmarshalCourseList([]byte(courseListJson))
	idList := PrintCourseTableReturnIDList(parsedCourseList)
	var cookies []*http.Cookie

	// Prompt user to select a course index
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Select course index: ")
	idxStr, _ := reader.ReadString('\n')
	idxStr = strings.TrimSpace(idxStr)
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 || idx >= len(idList) {
		fmt.Println("Invalid index. Exiting.")
		return
	}

	entry := idList[idx]

	// show selected ids
	fmt.Printf("Selected: %s courseID=%s classroomID=%s\n", entry.Name, entry.CourseID, entry.ClassroomID)

	// Fetch chapters from server (no local chapters file)
	chaptersFilename := fmt.Sprintf("chapters_%s_%s.json", entry.CourseID, entry.ClassroomID)
	var chaptersJson string
	if false { // disabled: no local chapters file read
		data, err := os.ReadFile(chaptersFilename)
		if err != nil {
			fmt.Println("Failed to read local chapters file:", err)
			return
		}
		chaptersJson = string(data)
	} else {
		// Local chapters file not found — try local cookie file before prompting
		// local chapters file reading disabled

		cookieFilename := "cookie.txt"
		var cookieRaw string
		if _, err := os.Stat(cookieFilename); err == nil {
			data, err := os.ReadFile(cookieFilename)
			if err == nil {
				cookieRaw = strings.TrimSpace(string(data))
				fmt.Println("Using cookie from", cookieFilename)
			} else {
				fmt.Println("Failed to read cookie file:", err)
			}
		}

		if cookieRaw == "" {
			// fallbaidListck: ask user to paste cookie
			fmt.Println("To fetch chapters from the server, paste your raw cookie string and press Enter (leave empty to abort):")

			cookieRaw, _ = reader.ReadString('\n')
			cookieRaw = strings.TrimSpace(cookieRaw)
			if cookieRaw == "" {
				fmt.Println("No cookie provided — aborting.")
				return
			}
		}

		cookies = ParseRawCookie(cookieRaw)
		fmt.Println(cookies)
		fmt.Println("https://www.yuketang.cn/v2/api/web/logs/learn/" + entry.ClassroomID)
		chaptersJson = GetChapters(entry.ClassroomID, cookies)
		fmt.Print(chaptersJson)
	}

	chapters, err := ParseChapters([]byte(chaptersJson))
	if err != nil {
		fmt.Println("Failed to parse chapters:", err)
		return
	}

	// Query watch-progress for each section and store status
	watchStatus := make(map[string]bool) // videoID -> completed
	for _, ch := range chapters {
		// sections may be []map[string]interface{} or []interface{}
		if sarr, ok := ch["sections"].([]map[string]interface{}); ok {
			for _, s := range sarr {
				idStr := idToString(s["id"])
				if idStr == "" {
					continue
				}
				done, raw, err := GetWatchProgressDetailed(entry.CourseID, entry.ClassroomID, idStr, cookies)
				if err != nil {
					fmt.Printf("[warn] watch progress check failed for %s: %v\n", idStr, err)
					watchStatus[idStr] = false
					continue
				}
				watchStatus[idStr] = done
				if raw != "" {
					fmt.Printf("[debug] id=%s raw response: %s\n", idStr, raw)
					if cval, found, perr := ParseCompletedFromRaw(raw, idStr); perr == nil && found {
						fmt.Printf("[debug] id=%s parsed completed=%d\n", idStr, cval)
					} else if perr != nil {
						fmt.Printf("[debug] id=%s parse error: %v\n", idStr, perr)
					}
				}
			}
		} else if sarr2, ok2 := ch["sections"].([]interface{}); ok2 {
			for _, sraw := range sarr2 {
				if s, ok := sraw.(map[string]interface{}); ok {
					idStr := idToString(s["id"])
					if idStr == "" {
						continue
					}
					done, raw, err := GetWatchProgressDetailed(entry.CourseID, entry.ClassroomID, idStr, cookies)
					if err != nil {
						fmt.Printf("[warn] watch progress check failed for %s: %v\n", idStr, err)
						watchStatus[idStr] = false
						continue
					}
					watchStatus[idStr] = done
					if raw != "" {
						fmt.Printf("[debug] id=%s raw response: %s\n", idStr, raw)
					}
				}
			}
		}
	}

	fmt.Printf("Chapters for course id %s (classroom %s):\n", entry.CourseID, entry.ClassroomID)
	for ci, ch := range chapters {
		cname := fmt.Sprintf("%v", ch["chapter_name"])
		fmt.Printf("%d. %s\n", ci, cname)

		// sections may be []map[string]interface{} or []interface{}
		if sarr, ok := ch["sections"].([]map[string]interface{}); ok {
			for si, s := range sarr {
				sname := fmt.Sprintf("%v", s["name"])
				sid := idToString(s["id"])
				status := ""
				if done, ok := watchStatus[sid]; ok {
					if done {
						status = "[DONE]"
					} else {
						status = "[TODO]"
					}
				}
				fmt.Printf("   %d.%d %s (id: %s) %s\n", ci, si, sname, sid, status)
			}
		} else if sarr2, ok2 := ch["sections"].([]interface{}); ok2 {
			for si, sraw := range sarr2 {
				if s, ok := sraw.(map[string]interface{}); ok {
					sname := fmt.Sprintf("%v", s["name"])
					sid := idToString(s["id"])
					status := ""
					if done, ok := watchStatus[sid]; ok {
						if done {
							status = "[DONE]"
						} else {
							status = "[TODO]"
						}
					}
					fmt.Printf("   %d.%d %s (id: %s) %s\n", ci, si, sname, sid, status)
				}
			}
		}
	}

	// After printing chapters, optionally run heartbeat iteration over all videos
	iterateAndHeartbeat(chapters, entry, cookies)

}

// iterateAndHeartbeat prompts user then sends heartbeat packets for each video in chapters
func iterateAndHeartbeat(chapters []map[string]interface{}, entry CourseEntry, cookies []*http.Cookie) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Apply heartbeat to all videos in this course? (y/N): ")
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))
	if ans != "y" && ans != "yes" {
		fmt.Println("Skipping heartbeat.")
		return
	}

	// determine user id
	userID := cookieValue(cookies, []string{"user_id", "userid", "uid"})
	if userID == "" {
		if uid, err := FetchUserID(cookies); err == nil {
			userID = uid
		} else {
			fmt.Printf("[warn] could not determine user_id: %v\n", err)
		}
	}

	// iterate chapters and sections
	for _, ch := range chapters {
		var sections []map[string]interface{}
		if sarr, ok := ch["sections"].([]map[string]interface{}); ok {
			sections = sarr
		} else if sarr2, ok2 := ch["sections"].([]interface{}); ok2 {
			for _, sraw := range sarr2 {
				if s, ok := sraw.(map[string]interface{}); ok {
					sections = append(sections, s)
				}
			}
		}
		for _, s := range sections {
			vidStr := idToString(s["id"])
			if vidStr == "" {
				continue
			}
			// try to get video_length via watch-progress
			_, raw, _ := GetWatchProgressDetailed(entry.CourseID, entry.ClassroomID, vidStr, cookies)
			totalSec := int64(60)
			if vl, ok := parseVideoLengthFromRaw(raw, vidStr); ok {
				totalSec = vl
			}
			// parse ids
			vidInt, _ := strconv.ParseInt(vidStr, 10, 64)
			cID := entry.CourseID
			cls := entry.ClassroomID
			// send heartbeats (default interval 60s)
			SendHeartbeatsForVideo(cID, cls, userID, vidInt, totalSec, 60, cookies)
		}
	}
}

// parseVideoLengthFromRaw extracts video_length (seconds) from raw response; returns (value, found)
func parseVideoLengthFromRaw(raw string, videoID string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}
	var root map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return 0, false
	}
	// look under data[videoID]
	if dataAny, ok := root["data"]; ok {
		if dataMap, ok := dataAny.(map[string]interface{}); ok {
			if vidAny, ok := dataMap[videoID]; ok {
				if vidMap, ok := vidAny.(map[string]interface{}); ok {
					if vl, ok := vidMap["video_length"]; ok {
						switch t := vl.(type) {
						case float64:
							return int64(t), true
						case string:
							if iv, err := strconv.ParseInt(t, 10, 64); err == nil {
								return iv, true
							}
						}
					}
				}
			}
		}
	}
	// try top-level
	if vidAny, ok := root[videoID]; ok {
		if vidMap, ok := vidAny.(map[string]interface{}); ok {
			if vl, ok := vidMap["video_length"]; ok {
				switch t := vl.(type) {
				case float64:
					return int64(t), true
				case string:
					if iv, err := strconv.ParseInt(t, 10, 64); err == nil {
						return iv, true
					}
				}
			}
		}
	}
	return 0, false
}

// SendHeartbeatsForVideo sends heartbeat GET requests to mark progress for a single video
func SendHeartbeatsForVideo(courseID string, classroomID string, userID string, videoID int64, totalSec int64, intervalSec int64, cookies []*http.Cookie) {
	if totalSec <= 0 {
		totalSec = int64(intervalSec)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	endpoint := "https://www.yuketang.cn/video-log/heartbeat/"

	// try to parse numeric course and user ids
	var courseNum int64
	if v, err := strconv.ParseInt(courseID, 10, 64); err == nil {
		courseNum = v
	}
	var userNum int64
	if v, err := strconv.ParseInt(userID, 10, 64); err == nil {
		userNum = v
	}

	for cp := int64(0); cp < totalSec; cp += int64(intervalSec) {
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
			"u":           userNum,
			"uip":         "",
			"c":           courseNum,
			"v":           videoID,
			"skuid":       13318608,
			"classroomid": classroomID,
			"cc":          "D8E356C1339D5C51B463AB73BD4C026B",
			"d":           0,
			"pg":          fmt.Sprintf("%d_ndl0", videoID),
			"sq":          1,
			"t":           "video",
			"cards_id":    0,
			"slide":       0,
			"v_url":       "",
		}
		body := map[string]interface{}{"heart_data": []interface{}{heart}}
		b, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(b))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; heartbeat-bot/1.0)")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("heartbeat error vid=%d cp=%d: %v", videoID, cp, err)
		} else {
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			log.Printf("sent heartbeat vid=%d cp=%d -> status=%s", videoID, cp, resp.Status)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// final packet at exact end
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
		"u":           userNum,
		"uip":         "",
		"c":           courseNum,
		"v":           videoID,
		"skuid":       13318608,
		"classroomid": classroomID,
		"cc":          "D8E356C1339D5C51B463AB73BD4C026B",
		"d":           0,
		"pg":          fmt.Sprintf("%d_ndl0", videoID),
		"sq":          1,
		"t":           "video",
		"cards_id":    0,
		"slide":       0,
		"v_url":       "",
	}
	body := map[string]interface{}{"heart_data": []interface{}{heart}}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(b))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; heartbeat-bot/1.0)")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("final heartbeat error vid=%d: %v", videoID, err)
	} else {
		_, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("final heartbeat vid=%d -> status=%s", videoID, resp.Status)
	}
}

func GetCourseList(Cookie []*http.Cookie) string {
	targetUrl := "https://www.yuketang.cn/v2/api/web/courses/list?identity=2"
	req, err := http.NewRequest("GET", targetUrl, nil)
	client := &http.Client{}

	if err != nil {
		fmt.Print(fmt.Errorf("couldn't create request object: %w", err))
	}

	for _, p := range Cookie {
		req.AddCookie(p)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("HTTP request failed:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Failed to read response body:", err)
		return ""
	}

	return string(body)

}

func ParseRawCookie(raw string) []*http.Cookie {
	raw = strings.TrimSpace(raw)
	// Allow pasting a full header line copied from devtools
	if strings.HasPrefix(strings.ToLower(raw), "cookie:") {
		raw = strings.TrimSpace(raw[len("cookie:"):])
	}

	parts := strings.Split(raw, ";")
	cookies := []*http.Cookie{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			// Skip attributes like Path, Domain if user pasted Set-Cookie fragments
			continue
		}
		name := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if name == "" {
			continue
		}
		cookies = append(cookies, &http.Cookie{
			Name:  name,
			Value: value,
		})
	}

	// Quick sanity hint in logs to help debug
	need := map[string]bool{"sessionid": false, "csrftoken": false}
	for _, c := range cookies {
		if _, ok := need[strings.ToLower(c.Name)]; ok {
			need[strings.ToLower(c.Name)] = true
		}
	}
	if !need["sessionid"] || !need["csrftoken"] {
		fmt.Println("[warn] cookie missing likely-required keys:", need)
	}
	return cookies
}
func UnmarshalCourseList(CouserListText []byte) map[string]interface{} {
	var ParsedCouserListText map[string]interface{}
	json.Unmarshal(CouserListText, &ParsedCouserListText)
	return ParsedCouserListText
}
func PrintCourseTableReturnIDList(ParsedCourseList map[string]interface{}) []CourseEntry {
	data, ok := ParsedCourseList["data"].(map[string]interface{})
	if !ok {
		fmt.Println("Failed to parse 'data' field")

		return make([]CourseEntry, 0)
	}
	list := data["list"]

	// You can now use 'list' as needed, e.g., print or process it
	var idList []CourseEntry
	for index, p := range list.([]interface{}) {
		row := p.(map[string]interface{})
		courseObj := row["course"].(map[string]interface{})

		// course name (may be string or nil)
		courseName := fmt.Sprintf("%v", courseObj["name"])

		// extract classroom_id
		var classroomID string
		switch v := row["classroom_id"].(type) {
		case string:
			classroomID = v
		case float64:
			classroomID = strconv.FormatInt(int64(v), 10)
		default:
			classroomID = fmt.Sprintf("%v", v)
		}

		// extract course id from courseObj
		var courseID string
		switch v := courseObj["id"].(type) {
		case string:
			courseID = v
		case float64:
			courseID = strconv.FormatInt(int64(v), 10)
		default:
			courseID = fmt.Sprintf("%v", v)
		}

		idList = append(idList, CourseEntry{CourseID: courseID, ClassroomID: classroomID, Name: courseName})
		fmt.Printf("%-2d %-40s courseID=%s classroomID=%s\n", index, courseName, courseID, classroomID)
	}
	return idList
}

func GetChapters(cid string, Cookie []*http.Cookie) string {
	targetUrl := fmt.Sprintf("https://www.yuketang.cn/mooc-api/v1/lms/learn/course/chapter?cid=%s&classroom_id=%s", cid, cid)

	client := &http.Client{}

	req, _ := http.NewRequest("GET", targetUrl, nil)
	// Some endpoints are picky about headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://www.yuketang.cn/")
	req.Header.Set("xtbz", "ykt")
	for _, p := range Cookie {
		req.AddCookie(p)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("HTTP request failed:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Failed to read response body:", err)
		return ""
	}
	if resp.StatusCode != 200 {
		fmt.Printf("[warn] non-200 status %d from chapters API, body head: %.200s\n", resp.StatusCode, string(body))
	}
	return string(body)

}
func ParseChapters(jsonBytes []byte) ([]map[string]interface{}, error) {
	// Root JSON object
	var root map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w; head=%.200s", err, string(jsonBytes))
	}

	// "data" layer
	dataAny, ok := root["data"]
	if !ok {
		return nil, fmt.Errorf("unexpected JSON: missing 'data'; keys=%v; head=%.200s", keysOf(root), string(jsonBytes))
	}
	data, ok := dataAny.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected JSON: 'data' not object, got %T; head=%.200s", dataAny, string(jsonBytes))
	}

	// "course_chapter" list
	ccAny, ok := data["course_chapter"]
	if !ok {
		return nil, fmt.Errorf("unexpected JSON: missing 'course_chapter'; data keys=%v; head=%.200s", keysOf(data), string(jsonBytes))
	}
	chapters, ok := ccAny.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected JSON: 'course_chapter' not array, got %T; head=%.200s", ccAny, string(jsonBytes))
	}

	// Final result to return
	result := make([]map[string]interface{}, 0, len(chapters))

	for _, ch := range chapters {
		chapter, ok := ch.(map[string]interface{})
		if !ok {
			// skip invalid item instead of panicking
			continue
		}

		chapterName := chapter["name"]
		chapterID := chapter["id"]

		// Parse "section_leaf_list"
		sectionList := make([]map[string]interface{}, 0)
		if sr, ok := chapter["section_leaf_list"]; ok {
			if sectionsRaw, ok := sr.([]interface{}); ok {
				for _, s := range sectionsRaw {
					if sec, ok := s.(map[string]interface{}); ok {
						secName := sec["name"]
						secID := sec["id"]
						sectionList = append(sectionList, map[string]interface{}{
							"name": secName,
							"id":   secID,
						})
					}
				}
			}
		}

		// Append each chapter entry into the result list
		result = append(result, map[string]interface{}{
			"chapter_name": chapterName,
			"chapter_id":   chapterID,
			"sections":     sectionList,
		})
	}

	return result, nil
}

// keysOf returns the keys of a map for concise debug output
func keysOf(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// cookieValue extracts a cookie value by trying multiple possible names.
func cookieValue(cookies []*http.Cookie, names []string) string {
	lower := make(map[string]string)
	for _, c := range cookies {
		lower[strings.ToLower(c.Name)] = c.Value
	}
	for _, n := range names {
		if v, ok := lower[strings.ToLower(n)]; ok && v != "" {
			return v
		}
	}
	return ""
}

func completedFromMap(m map[string]interface{}) (bool, error) {
	if v, ok := m["completed"]; ok {
		switch t := v.(type) {
		case float64:
			return int64(t) == 1, nil
		case int:
			return t == 1, nil
		case bool:
			return t, nil
		case string:
			return t == "1", nil
		default:
			return false, fmt.Errorf("unexpected completed type %T", v)
		}
	}
	return false, fmt.Errorf("completed field missing")
}

// FetchUserID requests the user info endpoint and extracts the user_id from the response.
func FetchUserID(Cookie []*http.Cookie) (string, error) {
	target := "https://www.yuketang.cn/v/course_meta/user_info"
	client := &http.Client{}
	req, _ := http.NewRequest("GET", target, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://www.yuketang.cn/")
	for _, p := range Cookie {
		req.AddCookie(p)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read user info body failed: %w", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return "", fmt.Errorf("invalid user info json: %w; head=%.200s", err, string(body))
	}

	dataAny, ok := root["data"]
	if !ok {
		return "", fmt.Errorf("user info missing data; head=%.200s", string(body))
	}
	data, ok := dataAny.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("user info data not object; head=%.200s", string(body))
	}
	upAny, ok := data["user_profile"]
	if !ok {
		return "", fmt.Errorf("user_profile missing; head=%.200s", string(body))
	}
	up, ok := upAny.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("user_profile not object; head=%.200s", string(body))
	}
	if uid, ok := up["user_id"]; ok {
		return idToString(uid), nil
	}
	return "", fmt.Errorf("user_id not found in user_profile; head=%.200s", string(body))
}

// GetWatchProgress queries the yuketang watch-progress endpoint for a single video id.
// GetWatchProgressDetailed queries the watch-progress endpoint and returns the completed flag and raw response body.
func GetWatchProgressDetailed(courseID string, classroomID string, videoID string, Cookie []*http.Cookie) (bool, string, error) {
	userID := cookieValue(Cookie, []string{"user_id", "userid", "uid"})
	cookieClassroom := cookieValue(Cookie, []string{"classroom_id", "classroomid", "classroom-id", "classroom"})

	// normalize userID if present in cookies (could be scientific-notation string)
	if userID != "" {
		userID = normalizeNumericString(userID)
	}

	if userID == "" {
		// try to fetch user_id from the user_info endpoint
		if uid, err := FetchUserID(Cookie); err == nil && uid != "" {
			userID = uid
		} else {
			return false, "", fmt.Errorf("user_id not found in cookies and user_info fetch failed: %v", err)
		}
	}

	// normalize classroomID from cookies if present; fallback to courseID
	if cookieClassroom != "" {
		classroomID = normalizeNumericString(cookieClassroom)
	}
	if classroomID == "" {
		classroomID = normalizeNumericString(courseID)
	}

	targetUrl := fmt.Sprintf("https://www.yuketang.cn/video-log/get_video_watch_progress/?cid=%s&user_id=%s&classroom_id=%s&video_type=video&vtype=rate&video_id=%s&snapshot=1", courseID, userID, classroomID, videoID)
	fmt.Printf("[debug] watch-progress request: user_id=%s classroom_id=%s video_id=%s url=%s\n", userID, classroomID, videoID, targetUrl)

	client := &http.Client{}
	req, _ := http.NewRequest("GET", targetUrl, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://www.yuketang.cn/")
	req.Header.Set("xtbz", "ykt")
	for _, p := range Cookie {
		req.AddCookie(p)
	}
	resp, err := client.Do(req)

	if err != nil {
		return false, "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	fmt.Println("get_progress:", string(body))

	if err != nil {
		return false, "", fmt.Errorf("read body failed: %w", err)
	}

	rawBody := string(body)

	// helper to format numeric values nicely
	formatNumber := func(v interface{}) string {
		switch t := v.(type) {
		case float64:
			if math.Trunc(t) == t {
				return strconv.FormatInt(int64(t), 10)
			}
			return strconv.FormatFloat(t, 'f', -1, 64)
		case int:
			return strconv.FormatInt(int64(t), 10)
		case int64:
			return strconv.FormatInt(t, 10)
		case string:
			return normalizeNumericString(t)
		default:
			return fmt.Sprintf("%v", v)
		}
	}

	// Try to parse JSON and extract completed + video_length if present.
	raw := fmt.Sprintf("status=%d body=%s", resp.StatusCode, rawBody)

	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		// return raw for debugging
		return false, raw, fmt.Errorf("invalid json: %w; head=%.200s", err, raw)
	}

	// local helper to extract completed and video_length from a map
	extractFromMap := func(vidMap map[string]interface{}) (bool, string, error) {
		// completed
		done, err := completedFromMap(vidMap)
		if err != nil {
			// completed field missing or invalid -> return error so caller can decide
			// but still try to extract video_length for debugging
			vlen := ""
			if vl, ok := vidMap["video_length"]; ok {
				vlen = formatNumber(vl)
			}
			if vlen != "" {
				// append video_length to raw for caller visibility
				raw = fmt.Sprintf("status=%d video_length=%s body=%s", resp.StatusCode, vlen, rawBody)
			}
			return false, vlen, err
		}
		// video_length (optional)
		vlen := ""
		if vl, ok := vidMap["video_length"]; ok {
			vlen = formatNumber(vl)
		}
		if vlen != "" {
			raw = fmt.Sprintf("status=%d video_length=%s body=%s", resp.StatusCode, vlen, rawBody)
		}
		return done, vlen, nil
	}

	if dataAny, ok := root["data"]; ok {
		if dataMap, ok := dataAny.(map[string]interface{}); ok {
			if vidAny, ok := dataMap[videoID]; ok {
				if vidMap, ok := vidAny.(map[string]interface{}); ok {
					done, _, err := extractFromMap(vidMap)
					return done, raw, err
				}
			}
		}
	}
	if vidAny, ok := root[videoID]; ok {
		if vidMap, ok := vidAny.(map[string]interface{}); ok {
			done, _, err := extractFromMap(vidMap)
			return done, raw, err
		}
	}

	// As a fallback, try to find video_length anywhere under data or root keyed by numeric-like keys
	// (some responses may include video info under top-level numeric keys)
	// We'll try to find a map that contains "completed" and "video_length"
	var fallbackVlen string
	foundCompleted := false
	var findInMap func(map[string]interface{})
	findInMap = func(m map[string]interface{}) {
		if foundCompleted {
			return
		}
		if _, ok := m["completed"]; ok {
			if vl, ok2 := m["video_length"]; ok2 {
				fallbackVlen = formatNumber(vl)
			}
			foundCompleted = true
		} else {
			for _, v := range m {
				if subm, ok := v.(map[string]interface{}); ok {
					findInMap(subm)
					if foundCompleted {
						return
					}
				}
			}
		}
	}
	findInMap(root)
	if fallbackVlen != "" {
		raw = fmt.Sprintf("status=%d video_length=%s body=%s", resp.StatusCode, fallbackVlen, rawBody)
	}

	return false, raw, fmt.Errorf("no progress info for video id %s; http status=%d; resp head=%.200s", videoID, resp.StatusCode, raw)
}

// idToString converts an interface{} ID (often float64 from JSON) into a plain integer string when appropriate.
func idToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// If value is whole number, format as integer to avoid scientific notation
		if math.Trunc(t) == t {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ParseCompletedFromRaw extracts the "completed" integer (0/1) for videoID from a raw response string.
// Returns (value, found, error). If JSON is invalid returns an error. If key not present returns found=false.
func ParseCompletedFromRaw(raw string, videoID string) (int, bool, error) {
	raw = strings.TrimSpace(raw)
	// strip any prefix like "status=200 body=" to get JSON
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}

	var root map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return 0, false, err
	}

	// helper to extract from a map[string]interface{}
	extract := func(m map[string]interface{}) (int, bool) {
		if v, ok := m["completed"]; ok {
			switch t := v.(type) {
			case float64:
				return int(int64(t)), true
			case int:
				return t, true
			case bool:
				if t {
					return 1, true
				}
				return 0, true
			case string:
				// try parse numeric string
				if t == "1" {
					return 1, true
				} else if t == "0" {
					return 0, true
				}
				if iv, err := strconv.Atoi(t); err == nil {
					return iv, true
				}
				return 0, true
			default:
				return 0, true
			}
		}
		return 0, false
	}

	if dataAny, ok := root["data"]; ok {
		if dataMap, ok := dataAny.(map[string]interface{}); ok {
			if vidAny, ok := dataMap[videoID]; ok {
				if vidMap, ok := vidAny.(map[string]interface{}); ok {
					if v, f := extract(vidMap); f {
						return v, true, nil
					}
				}
			}
		}
	}

	if vidAny, ok := root[videoID]; ok {
		if vidMap, ok := vidAny.(map[string]interface{}); ok {
			if v, f := extract(vidMap); f {
				return v, true, nil
			}
		}
	}

	return 0, false, nil
}

// normalizeNumericString tries to parse a string as a float and, when it
// represents a whole number, returns an integer string (avoids scientific notation).
// If parsing fails, returns the original string.
func normalizeNumericString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// try parse as float64
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		if math.Trunc(f) == f {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return s
}
