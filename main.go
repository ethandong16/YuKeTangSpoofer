package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
)

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

	cid := idList[idx]

	// Fetch chapters from server (no local chapters file)
	chaptersFilename := fmt.Sprintf("chapters_%s.json", cid)
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
		fmt.Println("https://www.yuketang.cn/v2/api/web/logs/learn/" + cid)
		chaptersJson = GetChapters(cid, cookies)
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
				done, raw, err := GetWatchProgressDetailed(cid, idStr, cookies)
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
					done, raw, err := GetWatchProgressDetailed(cid, idStr, cookies)
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

	fmt.Printf("Chapters for course id %s:\n", cid)
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
func PrintCourseTableReturnIDList(ParsedCourseList map[string]interface{}) []string {
	data, ok := ParsedCourseList["data"].(map[string]interface{})
	if !ok {
		fmt.Println("Failed to parse 'data' field")

		return make([]string, 0)
	}
	list := data["list"]

	// You can now use 'list' as needed, e.g., print or process it
	var idList []string
	for index, p := range list.([]interface{}) {
		row := p.(map[string]interface{})
		courseObj := row["course"].(map[string]interface{})

		// course name (may be string or nil)
		courseName := fmt.Sprintf("%v", courseObj["name"])

		// Use classroom_id for the chapters API (cid)
		var idStr string
		switch v := row["classroom_id"].(type) {
		case string:
			idStr = v
		case float64:
			idStr = strconv.FormatInt(int64(v), 10)
		default:
			idStr = fmt.Sprintf("%v", v)
		}

		idList = append(idList, idStr)
		fmt.Printf("%-2d %-10s\n", index, courseName)

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
func GetWatchProgressDetailed(cid string, videoID string, Cookie []*http.Cookie) (bool, string, error) {
	userID := cookieValue(Cookie, []string{"user_id", "userid", "uid"})
	classroomID := cookieValue(Cookie, []string{"classroom_id", "classroomid", "classroom-id", "classroom"})

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

	// normalize classroomID from cookies if present; fallback to cid
	if classroomID != "" {
		classroomID = normalizeNumericString(classroomID)
	}
	if classroomID == "" {
		classroomID = normalizeNumericString(cid)
	}

	targetUrl := fmt.Sprintf("https://www.yuketang.cn/video-log/get_video_watch_progress/?cid=%s&user_id=%s&classroom_id=%s&video_type=video&vtype=rate&video_id=%s&snapshot=1", cid, userID, classroomID, videoID)
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

	if err != nil {
		return false, "", fmt.Errorf("read body failed: %w", err)
	}

	rawBody := string(body)
	raw := fmt.Sprintf("status=%d body=%s", resp.StatusCode, rawBody)

	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		// return raw for debugging
		return false, raw, fmt.Errorf("invalid json: %w; head=%.200s", err, raw)
	}

	if dataAny, ok := root["data"]; ok {
		if dataMap, ok := dataAny.(map[string]interface{}); ok {
			if vidAny, ok := dataMap[videoID]; ok {
				if vidMap, ok := vidAny.(map[string]interface{}); ok {
					done, err := completedFromMap(vidMap)
					return done, raw, err
				}
			}
		}
	}
	if vidAny, ok := root[videoID]; ok {
		if vidMap, ok := vidAny.(map[string]interface{}); ok {
			done, err := completedFromMap(vidMap)
			return done, raw, err
		}
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
