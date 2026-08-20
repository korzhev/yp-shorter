package main

import (
	"fmt"
	"runtime"
	"time"
)

// Task содержит имя файла для конвертации
type Task struct {
	Filename string
}

// Queue - это очередь задач
type Queue struct {
	ch chan *Task
}

func NewQueue() *Queue {
	return &Queue{
		ch: make(chan *Task, 1),
	}
}

func (q *Queue) Push(t *Task) {
	// добавляем задачу в очередь
	q.ch <- t
}

func (q *Queue) PopWait() *Task {
	// получаем задачу
	return <-q.ch
}

type Resizer struct {
	Width  uint32
	Height uint32
}

func NewResizer(w, h uint32) *Resizer {
	return &Resizer{
		Width:  w,
		Height: h,
	}
}

func (r *Resizer) Resize(filename string) error {
	// пропустим реализацию
	time.Sleep(10 * time.Millisecond)
	return nil
}

type Worker struct {
	id      int
	queue   *Queue
	resizer *Resizer
}

func NewWorker(id int, queue *Queue, resizer *Resizer) *Worker {
	w := Worker{
		id:      id,
		queue:   queue,
		resizer: resizer,
	}
	return &w
}

func (w *Worker) Loop() {
	for {
		t := w.queue.PopWait()

		err := w.resizer.Resize(t.Filename)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}

		fmt.Printf("worker #%d resized %s\n", w.id, t.Filename)
	}
}

func main() {
	queue := NewQueue()

	for i := 0; i < runtime.NumCPU(); i++ {
		w := NewWorker(i, queue, NewResizer(1024, 1024))
		go w.Loop()
	}

	for i := 0; i < 50; i++ {
		imagefile := fmt.Sprintf("gopher%d.jpg", i)
		queue.Push(&Task{Filename: imagefile})
	}

	time.Sleep(time.Second)
}
