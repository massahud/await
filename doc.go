/*
Package await is a simple module for asynchronous waiting. Use await when
you need to wait for asynchronous tasks to complete before continuing normal
execution.

Example with polling function that returns an error:
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	poll := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("poll error")
	}
	if result != await.Func(ctx, 500*time.Microsecond, poll); result.Err != nil {
		return result.Err
	}

Example simultaneously polling multiple functions:
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	poll1 := func(ctx context.Context) (interface{}, error) {
		return "poll1 return", nil
	}
	poll2 := func(ctx context.Context) (interface{}, error) {
		return nil, fmt.Error("poll2 error")
	}
	polls := map[string]goawait.PollFunc{"poll1": poll1, "poll2": poll2}
	results := await.All(ctx, 200*time.Microsecond, polls)
	for name, result := range results {
		fmt.Println("Name:", name, "result:", result)
	}

Example polling until the first function returns:
	faster := func(ctx context.Context) (interface{}, error) {
		time.Sleep(time.Microsecond)
		return "I'm fast", nil
	}
	slower := func(ctx context.Context) (interface{}, error) {
		time.Sleep(time.Millisecond)
		return "I'm slow", nil
	}
	polls := map[string]goawait.PollFunc{"faster": faster, "slower": slower}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := await.First(ctx, time.Millisecond, polls)
	if result.Err != nil {
		log.Fatal(result.Err)
	}
	fmt.Prinln("first result:", result.Value)
*/
package await
