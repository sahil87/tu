import { describe, it, afterEach } from "node:test";
import assert from "node:assert/strict";
import { bold, dim, green, red, cyan, yellow, boldWhite, boldCyan, brightGreen, dimGreen, setNoColor } from "../colors.js";

afterEach(() => {
  setNoColor(false);
  delete process.env.NO_COLOR;
});

describe("color helpers", () => {
  it("bold wraps with ANSI bold code", () => {
    setNoColor(false);
    assert.equal(bold("hello"), "\x1b[1mhello\x1b[0m");
  });

  it("dim wraps with ANSI dim code", () => {
    setNoColor(false);
    assert.equal(dim("hello"), "\x1b[2mhello\x1b[0m");
  });

  it("green wraps with ANSI green code", () => {
    setNoColor(false);
    assert.equal(green("hello"), "\x1b[32mhello\x1b[0m");
  });

  it("red wraps with ANSI red code", () => {
    setNoColor(false);
    assert.equal(red("hello"), "\x1b[31mhello\x1b[0m");
  });

  it("cyan wraps with ANSI cyan code", () => {
    setNoColor(false);
    assert.equal(cyan("hello"), "\x1b[36mhello\x1b[0m");
  });

  it("yellow wraps with ANSI yellow code", () => {
    setNoColor(false);
    assert.equal(yellow("hello"), "\x1b[33mhello\x1b[0m");
  });

  it("boldWhite wraps with ANSI bold white code", () => {
    setNoColor(false);
    assert.equal(boldWhite("hello"), "\x1b[1;37mhello\x1b[0m");
  });

  it("boldCyan wraps with ANSI bold cyan code", () => {
    setNoColor(false);
    assert.equal(boldCyan("hello"), "\x1b[1;36mhello\x1b[0m");
  });

  it("brightGreen wraps with ANSI bright green code", () => {
    setNoColor(false);
    assert.equal(brightGreen("hello"), "\x1b[92mhello\x1b[0m");
  });

  it("dimGreen wraps with ANSI dim green code", () => {
    setNoColor(false);
    assert.equal(dimGreen("hello"), "\x1b[2;32mhello\x1b[0m");
  });
});

describe("NO_COLOR environment variable", () => {
  it("returns input unchanged when NO_COLOR is set", () => {
    process.env.NO_COLOR = "1";
    assert.equal(green("hello"), "hello");
    assert.equal(bold("hello"), "hello");
    assert.equal(boldWhite("hello"), "hello");
  });

  it("returns input unchanged when NO_COLOR is any truthy value", () => {
    process.env.NO_COLOR = "true";
    assert.equal(red("hello"), "hello");
  });
});

describe("setNoColor runtime flag", () => {
  it("disables color when set to true", () => {
    setNoColor(true);
    assert.equal(green("hello"), "hello");
    assert.equal(boldCyan("hello"), "hello");
    assert.equal(dim("hello"), "hello");
  });

  it("re-enables color when set to false", () => {
    setNoColor(true);
    assert.equal(green("hello"), "hello");
    setNoColor(false);
    assert.equal(green("hello"), "\x1b[32mhello\x1b[0m");
  });
});

describe("empty string handling", () => {
  it("wraps empty string", () => {
    setNoColor(false);
    assert.equal(green(""), "\x1b[32m\x1b[0m");
  });

  it("returns empty string when noColor", () => {
    setNoColor(true);
    assert.equal(green(""), "");
  });
});
