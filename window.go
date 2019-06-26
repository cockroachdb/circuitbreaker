package circuit

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/facebookgo/clock"
)

var (
	// DefaultWindowTime is the default time the window covers, 10 seconds.
	DefaultWindowTime = time.Millisecond * 10000

	// DefaultWindowBuckets is the default number of buckets the window holds, 10.
	DefaultWindowBuckets = 10
)

// bucket holds counts of failures and successes
type bucket struct {
	failure int64
	success int64
}

// Reset resets the counts to 0
func (b *bucket) Reset() {
	atomic.StoreInt64(&b.failure, 0)
	atomic.StoreInt64(&b.success, 0)
}

// Fail increments the failure count
func (b *bucket) Fail() {
	atomic.AddInt64(&b.failure, 1)
}

// Sucecss increments the success count
func (b *bucket) Success() {
	atomic.AddInt64(&b.success, 1)
}

// window maintains a ring of buckets and increments the failure and success
// counts of the current bucket. Once a specified time has elapsed, it will
// advance to the next bucket, reseting its counts. This allows the keeping of
// rolling statistics on the counts.
type window struct {
	buckets    []bucket
	bucketTime time.Duration
	bucketLock sync.Mutex
	lastAccess time.Time
	lastIdx    uint64
	clock      clock.Clock
}

// newWindow creates a new window. windowTime is the time covering the entire
// window. windowBuckets is the number of buckets the window is divided into.
// An example: a 10 second window with 10 buckets will have 10 buckets covering
// 1 second each.
func newWindow(windowTime time.Duration, windowBuckets int, clock clock.Clock) *window {
	buckets := make([]bucket, windowBuckets)
	bucketTime := time.Duration(windowTime.Nanoseconds() / int64(windowBuckets))
	return &window{
		buckets:    buckets,
		bucketTime: bucketTime,
		clock:      clock,
		lastAccess: clock.Now(),
	}
}

// Fail records a failure in the current bucket.
func (w *window) Fail() {
	w.bucketLock.Lock()
	b := w.getLatestBucket()
	w.bucketLock.Unlock()
	b.Fail()
}

// Success records a success in the current bucket.
func (w *window) Success() {
	w.bucketLock.Lock()
	b := w.getLatestBucket()
	w.bucketLock.Unlock()
	b.Success()
}

// Failures returns the total number of failures recorded in all buckets.
func (w *window) Failures() int64 {
	var failures int64
	for i := 0; i < len(w.buckets); i++ {
		b := &w.buckets[i]
		failures += atomic.LoadInt64(&b.failure)
	}
	return failures
}

// Successes returns the total number of successes recorded in all buckets.
func (w *window) Successes() int64 {
	var successes int64
	for i := 0; i < len(w.buckets); i++ {
		b := &w.buckets[i]
		successes += atomic.LoadInt64(&b.success)
	}
	return successes
}

// Successes returns the total number of all recorded in all buckets.
func (w *window) Total() int64 {
	var total int64
	for i := 0; i < len(w.buckets); i++ {
		b := &w.buckets[i]
		total += atomic.LoadInt64(&b.success) + atomic.LoadInt64(&b.failure)
	}
	return total
}

// ErrorRate returns the error rate calculated over all buckets, expressed as
// a floating point number (e.g. 0.9 for 90%)
func (w *window) ErrorRate() float64 {
	var total int64
	var failures int64

	for i := 0; i < len(w.buckets); i++ {
		b := &w.buckets[i]
		total += atomic.LoadInt64(&b.success)
		failures += atomic.LoadInt64(&b.failure)
	}

	total += failures
	if total == 0 {
		return 0.0
	}

	return float64(failures) / float64(total)
}

// Reset resets the count of all buckets.
func (w *window) Reset() {
	for i := 0; i < len(w.buckets); i++ {
		w.buckets[i].Reset()
	}
}

// getLatestBucket returns the current bucket. If the bucket time has elapsed
// it will move to the next bucket, resetting its counts and updating the last
// access time before returning it. getLatestBucket assumes that the caller has
// locked the bucketLock
func (w *window) getLatestBucket() *bucket {
	n := len(w.buckets)
	b := &w.buckets[w.lastIdx%uint64(n)]
	now := w.clock.Now()
	elapsed := now.Sub(w.lastAccess)

	if elapsed > w.bucketTime {
		// Reset the buckets between now and number of buckets ago. If
		// that is more that the existing buckets, reset all.
		for i := 0; i < n; i++ {
			w.lastIdx++
			b = &w.buckets[w.lastIdx%uint64(n)]
			b.Reset()
			elapsed = time.Duration(int64(elapsed) - int64(w.bucketTime))
			if elapsed < w.bucketTime {
				// Done resetting buckets.
				break
			}
		}
		w.lastAccess = now
	}
	return b
}
