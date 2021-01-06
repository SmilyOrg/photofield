export default class Simulation {

  constructor(options) {
    this.runs = options.runs;
    this.actions = options.actions;
    let actionStart = 0;
    this.actions.forEach(action => {
      action.start = actionStart;
      actionStart += action.duration;
    });
  }

  run(target) {
    this.target = target;
    this.promise = new Promise(resolve => {
      this.finish = resolve;
    });
    this.runResults = [];
    this.runIndex = -1;
    this.nextRun();
    return this.promise;
  }

  nextRun() {
    if (this.runIndex >= 0) {
      this.runResults.push({
        params: this.runs[this.runIndex],
        frames: this.frames,
      });
    }
    this.runIndex++;
    if (this.runIndex >= this.runs.length) {
      this.finish(this.runResults);
      return;
    }
    const run = this.runs[this.runIndex];
    this.initRun(run);
  }

  initRun(run) {
    for (const name in run) {
      if (Object.hasOwnProperty.call(run, name)) {
        const value = run[name];
        this.target[name] = value;
      }
    }
    this.runStartTime = performance.now();
    this.frameStartTime = this.runStartTime;
    this.frames = [];
    window.requestAnimationFrame(this.nextFrame.bind(this));
  }

  nextFrame() {
    const now = performance.now();
    const elapsed = now - this.runStartTime;
    const frameTime = now - this.frameStartTime;
    this.frameStartTime = now;
    const action = this.getAction(elapsed);
    this.applyAction(action, elapsed, frameTime);
    if (elapsed >= action.start + action.duration) {
      this.nextRun();
      return;
    }
    window.requestAnimationFrame(this.nextFrame.bind(this));
  }

  getAction(elapsed) {
    let action = null;
    for (let i = 0; i < this.actions.length; i++) {
      action = this.actions[i];
      if (elapsed < action.start + action.duration) {
        break;
      }
    }
    return action;
  }

  applyAction(action, elapsed, frameTime) {
    if (!action) return;
    const t = Math.max(0, Math.min(1, (elapsed - action.start) / action.duration));
    const scroll = action.scroll;
    let speed = 0;
    if (scroll) {
      let y = scroll.from;
      if (scroll.to !== undefined) {
        const distance = scroll.to - scroll.from;
        speed = distance * 1000 / action.duration;
        y = scroll.from + t * distance;
      }
      this.target.$refs.scroller.scroll(0, y);
    }
    this.frames.push([ elapsed, frameTime, t, speed ]);
  }




}