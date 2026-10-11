package catalog

import (
	"errors"
	"testing"
)

func TestItemScriptBatchesConsumeBeforeReadingMore(t *testing.T) {
	rows := make([]IndexEntry, 3*itemScriptBatchSize+7)
	for i := range rows {
		rows[i].ID = uint32(i + 1)
	}
	consumed, fetched, calls := 0, 0, 0
	err := visitItemScriptBatches(rows, func(batch []IndexEntry) []itemScriptRead {
		if consumed != fetched {
			t.Fatalf("read next batch with %d unconsumed scripts retained", fetched-consumed)
		}
		if len(batch) > itemScriptBatchSize {
			t.Fatalf("unbounded batch: %d", len(batch))
		}
		calls++
		fetched += len(batch)
		out := make([]itemScriptRead, len(batch))
		for i := range batch {
			out[i].raw = []byte{byte(batch[i].ID)}
		}
		return out
	}, func(row IndexEntry, read itemScriptRead) error {
		if row.ID != uint32(consumed+1) || read.raw[0] != byte(row.ID) {
			t.Fatalf("native LIST order or row/read binding changed at row %d", row.ID)
		}
		consumed++
		return nil
	})
	if err != nil || consumed != len(rows) || calls != 4 {
		t.Fatalf("err=%v consumed=%d batches=%d", err, consumed, calls)
	}
}

func TestItemScriptBatchesStopAfterConsumerError(t *testing.T) {
	rows := make([]IndexEntry, 4*itemScriptBatchSize)
	want := errors.New("consumer refused script")
	fetched, consumed := 0, 0
	err := visitItemScriptBatches(rows, func(batch []IndexEntry) []itemScriptRead {
		fetched += len(batch)
		return make([]itemScriptRead, len(batch))
	}, func(IndexEntry, itemScriptRead) error {
		consumed++
		if consumed == itemScriptBatchSize+1 {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) || fetched != 2*itemScriptBatchSize {
		t.Fatalf("read scripts beyond refused batch: err=%v fetched=%d", err, fetched)
	}
}

func TestItemScriptBatchesNoReadRequired(t *testing.T) {
	rows := make([]IndexEntry, itemScriptBatchSize+3)
	consumed := 0
	err := visitItemScriptBatches(rows, func([]IndexEntry) []itemScriptRead { return nil }, func(_ IndexEntry, read itemScriptRead) error {
		if read.raw != nil || read.cells != nil || read.err != nil {
			t.Fatal("metadata-only import acquired a script")
		}
		consumed++
		return nil
	})
	if err != nil || consumed != len(rows) {
		t.Fatalf("metadata-only rows changed: err=%v consumed=%d", err, consumed)
	}
}
