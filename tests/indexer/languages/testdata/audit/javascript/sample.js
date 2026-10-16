export function top() {}

export const arrow = (a) => a + 1;

export const CONSTANT = 42;

const fnExpr = function () {};

class Base {
  static count = 0;
  name = "x";
  #secret = 1;

  constructor(name) {
    this.name = name;
  }

  get label() {
    return this.name;
  }

  static make() {
    return new Base("m");
  }

  handle = (e) => {};
}

module.exports = { top, Base };
module.exports.helper = function () {};
exports.other = () => {};

export default Base;
