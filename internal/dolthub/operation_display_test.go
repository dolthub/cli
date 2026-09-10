package dolthub

import "testing"

func TestOperationDisplay(t *testing.T) {
	const uuid = "716a6b3f-4bd4-432e-b7ae-87bead012a3f"
	for _, id := range []string{uuid, "operations/" + uuid, "repositoryOwners/dolthub/repositories/people/jobs/" + uuid} {
		op := Operation{ID: id, Status: OperationSucceeded}
		ref := OperationRef{ID: id, Href: "https://example.test/api/v2/operations/" + id}
		if got := op.ForDisplay(); got.ID != uuid || got.Status != op.Status || op.ID != id {
			t.Fatalf("operation display: %#v, original: %#v", got, op)
		}
		if got := ref.ForDisplay(); got.ID != uuid || got.Href != ref.Href || ref.ID != id {
			t.Fatalf("reference display: %#v, original: %#v", got, ref)
		}
	}
	for _, id := range []string{"", "pending", "job/"} {
		if got := ShortOperationID(id); got != id {
			t.Errorf("ShortOperationID(%q) = %q", id, got)
		}
	}
}
