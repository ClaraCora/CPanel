import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ConfirmDialog, Drawer } from "./ui";

describe("modal focus behavior", () => {
  it("traps focus inside a drawer and returns it to the opener", () => {
    function Example() {
      const [open, setOpen] = useState(false);
      return <><button type="button" onClick={() => setOpen(true)}>打开</button><Drawer open={open} title="编辑服务器" onClose={() => setOpen(false)}><input aria-label="服务器名称" /><button type="button">保存</button></Drawer></>;
    }

    render(<Example />);
    const opener = screen.getByRole("button", { name: "打开" });
    opener.focus();
    fireEvent.click(opener);
    const close = screen.getByRole("button", { name: "关闭对话框" });
    const save = screen.getByRole("button", { name: "保存" });
    expect(document.activeElement).toBe(close);
    save.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(document.activeElement).toBe(close);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("keeps a loading confirmation open when Escape is pressed", () => {
    const close = vi.fn();
    render(<ConfirmDialog open loading title="删除记录" description="操作不可撤销" onClose={close} onConfirm={vi.fn()} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(close).not.toHaveBeenCalled();
    expect(screen.getByRole("alertdialog")).toBeTruthy();
  });
});
