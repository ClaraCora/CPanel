import { Moon, Sun } from "lucide-react";
import { useTheme } from "../theme";

export function ThemeToggle({ className = "" }: { className?: string }) {
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === "dark";
  const label = isDark ? "切换为浅色模式" : "切换为深色模式";

  return <button type="button" className={`icon-button theme-toggle ${className}`.trim()} aria-label={label} aria-pressed={isDark} title={label} onClick={toggleTheme}>
    {isDark ? <Sun size={17} aria-hidden="true" /> : <Moon size={17} aria-hidden="true" />}
  </button>;
}
