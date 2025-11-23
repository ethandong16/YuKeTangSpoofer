package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	PrintCourseTableReturnIDList(parsedCourseList)

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
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	return string(body)

}

func ParseRawCookie(raw string) []*http.Cookie {

	parts := strings.Split(raw, ";")
	cookies := []*http.Cookie{}
	fmt.Println(len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		kv := strings.SplitN(p, "=", 2)

		if len(kv) != 2 {
			continue
		}

		cookies = append(cookies, &http.Cookie{
			Name:  kv[0],
			Value: kv[1],
		})
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
		courseName := row["course"].(map[string]interface{})["name"].(string)
		id := row["course"].(map[string]interface{})["id"].(string)

		idList = append(idList, id)
		fmt.Printf("%-2d %-10s\n", index, courseName)

	}
	return idList
}

func GetChapters(cid string, Cookie []*http.Cookie) string {
	targetUrl := "https://www.yuketang.cn/mooc-api/v1/lms/learn/course/chapter?cid=" + cid
	client := &http.Client{}

	req, _ := http.NewRequest("GET", targetUrl, nil)
	for _, p := range Cookie {
		req.AddCookie(p)
	}
	resp, _ := client.Do(req)

	body, _ := io.ReadAll(resp.Body)
	return string(body)

}
func ParseChapters(jsonBytes []byte) ([]map[string]interface{}, error) {
	// Root JSON object
	var root map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &root); err != nil {
		return nil, err
	}

	// "data" layer
	data := root["data"].(map[string]interface{})

	// "course_chapter" list
	chapters := data["course_chapter"].([]interface{})

	// Final result to return
	result := make([]map[string]interface{}, 0)

	for _, ch := range chapters {
		chapter := ch.(map[string]interface{})

		chapterName := chapter["name"]
		chapterID := chapter["id"]

		// Parse "section_leaf_list"
		sectionsRaw := chapter["section_leaf_list"].([]interface{})
		sectionList := make([]map[string]interface{}, 0)

		for _, s := range sectionsRaw {
			sec := s.(map[string]interface{})

			secName := sec["name"]
			secID := sec["id"]

			sectionList = append(sectionList, map[string]interface{}{
				"name": secName,
				"id":   secID,
			})
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
