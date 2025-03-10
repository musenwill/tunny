// Copyright (c) 2014 Ashley Jeffs
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package tunny

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

//------------------------------------------------------------------------------

func TestPoolSizeAdjustment(t *testing.T) {
	pool := NewFunc(10, func(interface{}) interface{} { return "foo" })
	if exp, act := 10, len(pool.workers); exp != act {
		t.Errorf("Wrong size of pool: %v != %v", act, exp)
	}

	pool.SetSize(10)
	if exp, act := 10, pool.GetSize(); exp != act {
		t.Errorf("Wrong size of pool: %v != %v", act, exp)
	}

	pool.SetSize(9)
	if exp, act := 9, pool.GetSize(); exp != act {
		t.Errorf("Wrong size of pool: %v != %v", act, exp)
	}

	pool.SetSize(10)
	if exp, act := 10, pool.GetSize(); exp != act {
		t.Errorf("Wrong size of pool: %v != %v", act, exp)
	}

	pool.SetSize(0)
	if exp, act := 0, pool.GetSize(); exp != act {
		t.Errorf("Wrong size of pool: %v != %v", act, exp)
	}

	pool.SetSize(10)
	if exp, act := 10, pool.GetSize(); exp != act {
		t.Errorf("Wrong size of pool: %v != %v", act, exp)
	}

	// Finally, make sure we still have actual active workers.
	ret, err := pool.Process(0)
	if exp, act := "foo", ret.(string); err != nil || exp != act {
		t.Errorf("Wrong result: %v != %v, err: %s", act, exp, err)
	}

	pool.Close()
	if exp, act := 0, pool.GetSize(); exp != act {
		t.Errorf("Wrong size of pool: %v != %v", act, exp)
	}
}

//------------------------------------------------------------------------------

func TestFuncJob(t *testing.T) {
	pool := NewFunc(10, func(in interface{}) interface{} {
		intVal := in.(int)
		return intVal * 2
	})
	defer pool.Close()

	for i := 0; i < 10; i++ {
		ret, err := pool.Process(10)
		if exp, act := 20, ret.(int); err != nil || exp != act {
			t.Errorf("Wrong result: %v != %v, err: %v", act, exp, err)
		}
	}
}

func TestFuncJobAsync(t *testing.T) {
	pool := NewFunc(10, func(in interface{}) interface{} {
		intVal := in.(int)
		return intVal * 2
	})
	defer pool.Close()

	payloads := make(chan interface{}, 10)
	results := make(chan interface{}, 10)

	pool.AsyncProcess(payloads, results)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for ret := range results {
			if exp, act := 20, ret.(int); exp != act {
				t.Errorf("Wrong result: %v != %v", act, exp)
			}
		}
	}()

	for i := 0; i < 10; i++ {
		payloads <- 10
	}
	close(payloads)

	wg.Wait()
}

func TestFuncJobTimed(t *testing.T) {
	pool := NewFunc(10, func(in interface{}) interface{} {
		intVal := in.(int)
		return intVal * 2
	})
	defer pool.Close()

	for i := 0; i < 10; i++ {
		ret, err := pool.ProcessTimed(10, time.Millisecond)
		if exp, act := 20, ret.(int); err != nil || exp != act {
			t.Errorf("Wrong result: %v != %v, err: %v", act, exp, err)
		}
	}
}

func TestCallbackJob(t *testing.T) {
	pool := NewCallback(10)
	defer pool.Close()

	var counter int32
	for i := 0; i < 10; i++ {
		ret, err := pool.Process(func() {
			atomic.AddInt32(&counter, 1)
		})
		if ret != nil || err != nil {
			t.Errorf("Non-nil callback response: %v, err: %v", ret, err)
		}
	}

	ret, err := pool.Process("foo")
	if exp, act := ErrJobNotFunc, ret; err != nil || exp != act {
		t.Errorf("Wrong result from non-func: %v != %v, err: %v", act, exp, err)
	}

	if exp, act := int32(10), counter; exp != act {
		t.Errorf("Wrong result: %v != %v", act, exp)
	}
}

func TestTimeout(t *testing.T) {
	pool := NewFunc(1, func(in interface{}) interface{} {
		intVal := in.(int)
		<-time.After(time.Millisecond)
		return intVal * 2
	})
	defer pool.Close()

	_, act := pool.ProcessTimed(10, time.Duration(1))
	if exp := ErrJobTimedOut; exp != act {
		t.Errorf("Wrong error returned: %v != %v", act, exp)
	}
}

func TestTimedJobsAfterClose(t *testing.T) {
	pool := NewFunc(1, func(in interface{}) interface{} {
		return 1
	})
	pool.Close()

	_, act := pool.ProcessTimed(10, time.Duration(1))
	if exp := ErrPoolNotRunning; exp != act {
		t.Errorf("Wrong error returned: %v != %v", act, exp)
	}
}

func TestJobsAfterClose(t *testing.T) {
	pool := NewFunc(1, func(in interface{}) interface{} {
		return 1
	})
	pool.Close()

	_, err := pool.Process(10)
	if exp, act := ErrPoolNotRunning, err; exp != act {
		t.Errorf("Process after Stop() returned: %v != %v", act, exp)
	}
}

func TestParallelJobs(t *testing.T) {
	nWorkers := 10

	jobGroup := sync.WaitGroup{}
	testGroup := sync.WaitGroup{}

	pool := NewFunc(nWorkers, func(in interface{}) interface{} {
		jobGroup.Done()
		jobGroup.Wait()

		intVal := in.(int)
		return intVal * 2
	})
	defer pool.Close()

	for j := 0; j < 1; j++ {
		jobGroup.Add(nWorkers)
		testGroup.Add(nWorkers)

		for i := 0; i < nWorkers; i++ {
			go func() {
				ret, err := pool.Process(10)
				if exp, act := 20, ret.(int); err != nil || exp != act {
					t.Errorf("Wrong result: %v != %v, err: %v", act, exp, err)
				}
				testGroup.Done()
			}()
		}

		testGroup.Wait()
	}
}

//------------------------------------------------------------------------------

type mockWorker struct {
	blockProcChan  chan struct{}
	blockReadyChan chan struct{}
	interruptChan  chan struct{}
	terminated     bool
}

func (m *mockWorker) Process(in interface{}) interface{} {
	select {
	case <-m.blockProcChan:
	case <-m.interruptChan:
	}
	return in
}

func (m *mockWorker) BlockUntilReady() {
	<-m.blockReadyChan
}

func (m *mockWorker) Interrupt() {
	m.interruptChan <- struct{}{}
}

func (m *mockWorker) Terminate() {
	m.terminated = true
}

func TestCustomWorker(t *testing.T) {
	pool := New(1, func() Worker {
		return &mockWorker{
			blockProcChan:  make(chan struct{}),
			blockReadyChan: make(chan struct{}),
			interruptChan:  make(chan struct{}),
		}
	})

	worker1, ok := pool.workers[0].worker.(*mockWorker)
	if !ok {
		t.Fatal("Wrong type of worker in pool")
	}

	if worker1.terminated {
		t.Fatal("Worker started off terminated")
	}

	_, err := pool.ProcessTimed(10, time.Millisecond)
	if exp, act := ErrJobTimedOut, err; exp != act {
		t.Errorf("Wrong error: %v != %v", act, exp)
	}

	close(worker1.blockReadyChan)
	_, err = pool.ProcessTimed(10, time.Millisecond)
	if exp, act := ErrJobTimedOut, err; exp != act {
		t.Errorf("Wrong error: %v != %v", act, exp)
	}

	close(worker1.blockProcChan)
	ret, err := pool.Process(10)
	if exp, act := 10, ret.(int); err != nil || exp != act {
		t.Errorf("Wrong result: %v != %v, err: %v", act, exp, err)
	}

	pool.Close()
	if !worker1.terminated {
		t.Fatal("Worker was not terminated")
	}
}

//------------------------------------------------------------------------------

func BenchmarkFuncJob(b *testing.B) {
	pool := NewFunc(10, func(in interface{}) interface{} {
		fVal := in.(float64)
		for i := 0; i < 100000; i++ {
			fVal *= 1.001
		}
		return fVal
	})
	defer pool.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := pool.Process(1.0)
		if err != nil {
			b.Errorf("Wrong result: err: %v", err)
		}
	}
}

func BenchmarkFuncTimedJob(b *testing.B) {
	pool := NewFunc(10, func(in interface{}) interface{} {
		fVal := in.(float64)
		for i := 0; i < 100000; i++ {
			fVal *= 1.001
		}
		return fVal
	})
	defer pool.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := pool.ProcessTimed(1.0, time.Second)
		if err != nil {
			b.Errorf("Wrong result: err: %v", err)
		}
	}
}

//------------------------------------------------------------------------------
