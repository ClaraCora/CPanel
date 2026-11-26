type EditorSection = { id: string; title: string };

export function EditorSectionNav({ label, sections, current, showTitle, onSelect }: {
  label: string;
  sections: EditorSection[];
  current: string;
  showTitle: boolean;
  onSelect: (id: string) => void;
}) {
  function jumpTo(id: string) {
    const section = document.getElementById(id);
    if (!section) return;
    onSelect(id);
    section.querySelector("h2")?.focus({ preventScroll: true });
    section.scrollIntoView({
      block: "start",
      behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth",
    });
  }

  return <nav className="editor-section-nav" aria-label={label}>
    {showTitle && <h2 className="editor-index-title">编辑目录</h2>}
    {sections.map(({ id, title }) => <button type="button" key={id} aria-controls={id}
      aria-current={current === id ? "location" : undefined} onClick={() => jumpTo(id)}>
      {title}
    </button>)}
  </nav>;
}
