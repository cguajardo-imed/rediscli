# Progress Bar for Iterations

## Overview

When running operations with multiple iterations, the Redis CLI tool now displays a **live progress bar** that updates in real-time as each iteration completes. This provides immediate visual feedback and helps you track the progress of long-running operations.

## Features

- **Real-time Updates**: Progress bar updates after each iteration completes
- **Percentage Display**: Shows exact completion percentage
- **Iteration Counter**: Displays current iteration vs. total (e.g., "5/10")
- **Status Messages**: Shows what's currently happening
- **Time Tracking**: 
  - Elapsed time since operation started
  - Estimated time remaining based on average iteration time
- **Colorful Visual**: Uses gradient colors for an attractive display

## Progress Bar Components

```
Operation: Publish Create

█████████████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░

Progress: 5/10 iterations (50%)

Processing iteration 5/10...

Elapsed: 5.2s
Estimated remaining: 5.1s

Please wait...
```

### Breakdown:

1. **Operation Name**: Shows which operation is running
   - "Publish Create" 
   - "Publish Create & Delete"

2. **Progress Bar**: Visual representation using filled (█) and empty (░) characters
   - 71 characters wide
   - Uses gradient coloring from purple to green
   - Fills from left to right

3. **Progress Counter**: Shows completed vs. total iterations
   - Format: `X/Y iterations (Z%)`
   - Example: `5/10 iterations (50%)`

4. **Status Message**: Current activity
   - "Starting operations..."
   - "Processing iteration X/Y..."

5. **Time Information**:
   - **Elapsed**: Total time since operation started
   - **Estimated remaining**: Calculated based on average time per iteration
   - Updates dynamically with each iteration

## How It Works

### 1. Start Operation
When you select "Publish create" or "Publish create & delete" and confirm the iteration count and delay:

```
Operation: Publish Create

░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░

Progress: 0/10 iterations (0%)

Starting operations...

Elapsed: 0s

Please wait...
```

### 2. During Execution
As each iteration completes, the progress bar fills up:

```
Operation: Publish Create

█████████████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░

Progress: 5/10 iterations (50%)

Processing iteration 6/10...

Elapsed: 6.3s
Estimated remaining: 6.2s

Please wait...
```

### 3. Completion
When all iterations finish, you see the results:

```
Query Result:

✓ Iteration 1/10: Published create event
✓ Iteration 2/10: Published create event
✓ Iteration 3/10: Published create event
...
✓ Iteration 10/10: Published create event

✓ Completed 10 iterations successfully!
  Duration: 12.5s
  Successful: 10 | Failed: 0

Log file: logs/rediscli_2026-02-19_12-22-06.log

enter/any key: continue • esc: main menu
```

## Time Estimation Algorithm

The estimated remaining time is calculated using a simple average:

```
Average Time Per Iteration = Total Elapsed Time / Completed Iterations
Estimated Remaining = Average Time Per Iteration × Remaining Iterations
```

### Example:
- 5 iterations completed in 10 seconds
- Average: 10s / 5 = 2 seconds per iteration
- Remaining: 5 iterations × 2s = 10 seconds estimated

The estimate becomes more accurate as more iterations complete.

## Technical Details

### Implementation
- Uses Bubble Tea's message-based architecture
- Progress updates sent via Go channels
- Non-blocking goroutine for operation execution
- Real-time UI updates without freezing

### Message Flow
```
1. User confirms iterations/delay
2. Operation starts in goroutine
3. Progress messages sent via channel
4. UI updates on each message
5. Final result displayed when complete
```

### Performance
- Minimal overhead (< 1ms per update)
- Efficient channel-based communication
- Does not slow down Redis operations

## User Experience Benefits

### 1. Visual Feedback
- No more wondering if the application is frozen
- Clear indication of progress
- Professional, polished appearance

### 2. Time Awareness
- Know exactly how long operations are taking
- Estimate when operations will complete
- Plan accordingly for large batch operations

### 3. Error Detection
- If progress stops, you know something is wrong
- Can cancel (Ctrl+C) if needed
- See exactly which iteration had issues (in logs)

## Examples

### Short Operation (3 iterations, 1s delay)
```
Operation: Publish Create

████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░

Progress: 2/3 iterations (67%)

Processing iteration 3/3...

Elapsed: 2.5s
Estimated remaining: 1.2s

Please wait...
```

### Long Operation (100 iterations, 500ms delay)
```
Operation: Publish Create & Delete

███████████████████████████████████████████████░░░░░░░░░░░░░░░░░░░░░░░░░

Progress: 68/100 iterations (68%)

Processing iteration 69/100...

Elapsed: 2m 14s
Estimated remaining: 1m 3s

Please wait...
```

## Comparison: Before vs. After

### Before (Without Progress Bar)
```
[Select "Publish create"]
[Enter 100]
[Enter 1s]

... waiting ...
... no feedback ...
... is it working? ...
... 2 minutes later ...

✓ Completed 100 iterations!
```

### After (With Progress Bar)
```
[Select "Publish create"]
[Enter 100]
[Enter 1s]

Operation: Publish Create
████████████░░░░░░░░ 12/100 (12%)
Elapsed: 14s | Est. remaining: 1m 42s

[Clear progress indication]
[Know exactly what's happening]
[Can estimate completion time]
```

## Tips

### For Large Batches
- Progress bar is especially useful for 50+ iterations
- Estimated time helps you decide if you want to wait or cancel
- Can run in background and check periodically

### For Quick Operations
- Even with few iterations, progress bar provides professional feel
- Shows the application is responsive
- Confirms operations are executing

### Monitoring
- Open the log file in another terminal with `tail -f` for detailed info
- Progress bar shows high-level status
- Logs show operation-level details

## Keyboard Controls

While progress bar is showing:
- **Ctrl+C**: Cancel operation and exit
- Other keys are disabled until operation completes

After completion:
- **Enter**: Return to main menu
- **Esc**: Return to main menu
- **Any key**: Clear results and return to menu

## Related Features

- **Logging**: All operations logged to file (see LOGGING.md)
- **Iteration Input**: Flexible iteration count (1 to unlimited)
- **Delay Input**: Customizable delay between iterations
- **Results Summary**: Detailed statistics after completion

## Future Enhancements (Potential)

- Pause/Resume functionality
- Real-time throughput display (ops/second)
- Progress bar color themes
- Option to hide/show estimated time
- CSV export of timing data