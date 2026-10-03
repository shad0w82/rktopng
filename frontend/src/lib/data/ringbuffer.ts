/** Fixed-size history of numbers: pushing drops the oldest. Memory only, no persistence. */
export class RingBuffer {
  private data: number[]
  private head = 0 // index of the oldest value

  /** Starts full of `fill` so charts draw a flat line instead of a ramp from zero. */
  constructor(readonly capacity: number, fill = 0) {
    this.data = new Array<number>(capacity).fill(fill)
  }

  push(v: number): void {
    this.data[this.head] = v
    this.head = (this.head + 1) % this.capacity
  }

  /** Values from the oldest to the newest. */
  toArray(): number[] {
    return [...this.data.slice(this.head), ...this.data.slice(0, this.head)]
  }

  /** The newest value. */
  last(): number {
    return this.data[(this.head - 1 + this.capacity) % this.capacity]
  }
}
