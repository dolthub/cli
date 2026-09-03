package dolthub

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUpdatePullRequestPreservesFieldPresence(t *testing.T) {
	empty := ""
	request := UpdatePullRequest{Description: &empty}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"description":""}`; got != want {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}

func TestQueryResultDecodesNullCells(t *testing.T) {
	var result QueryResult
	err := json.Unmarshal([]byte(`{"columns":[{"name":"n","type":"BIGINT"}],"rows":[[null],["42"]],"status":"success"}`), &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != QuerySuccess || result.Rows[0][0] != nil || result.Rows[1][0] == nil || *result.Rows[1][0] != "42" {
		t.Fatalf("result = %#v", result)
	}
}

func TestImportRequestUsesOpenAPIFieldNames(t *testing.T) {
	request := CreateImportRequest{
		BranchName:      "main",
		TableName:       "states",
		FileName:        "states.csv",
		FileSize:        10,
		FileType:        ImportCSV,
		ImportOperation: ImportCreate,
		Token:           "token",
		ContentsKey:     "contents",
		CompletedParts:  []CompletedPart{{PartNumber: 1, ETag: "etag"}},
		FilePartsMD5:    "md5",
		PrimaryKeys:     []string{},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"branch_name"`, `"import_operation"`, `"completed_parts"`, `"file_parts_md5"`, `"primary_keys":[]`} {
		if !strings.Contains(string(encoded), field) {
			t.Errorf("JSON %s does not contain %s", encoded, field)
		}
	}
}
