export enum Direction {
  Up,
  Down,
}

export const enum Flags {
  A = 1,
}

declare function ambient(x: number): void;

export namespace Util {
  export function helper(): void {}
}

declare module "ext" {
  export const x: number;
}

export abstract class Base<T> {
  static count = 0;
  readonly id: string = "";
  private cache?: Map<string, T>;
  name: string;

  constructor(name: string) {
    this.name = name;
  }

  abstract run(): void;

  get label(): string {
    return this.name;
  }

  handle = (e: Event): void => {};
}

export interface Repo<T> {
  find(id: string): T;
  readonly size: number;
  name: string;
}

export type Id = string | number;

export const arrow = (a: number): number => a + 1;

export const fnExpr = function (a: number) {
  return a;
};

export const CONSTANT = 42;

export default class Main {}
