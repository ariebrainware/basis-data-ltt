package docs

import (
    "testing"
    "strings"
)

func TestSwaggerInfoBasic(t *testing.T) {
    if SwaggerInfo == nil {
        t.Fatalf("SwaggerInfo unexpectedly nil")
    }
    if SwaggerInfo.Title == "" {
        t.Fatalf("expected non-empty Title in SwaggerInfo")
    }
    if !strings.Contains(SwaggerInfo.SwaggerTemplate, "paths") {
        t.Fatalf("expected SwaggerTemplate to contain 'paths'")
    }
}
