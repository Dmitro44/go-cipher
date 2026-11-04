// Оформление пояснительных записок (спек) к лабораторным работам по МЗИ.
// Формат произвольный: задача документа -- быть читаемым конспектом для
// подготовки к защите, а не отчётом по СТП.

#let accent = rgb("#1f4e79")
#let soft = rgb("#eef3f8")

#let spec(title: "", subtitle: "", source: "", body) = {
  set document(title: title)
  set page(
    paper: "a4",
    margin: (x: 2cm, y: 2cm),
    numbering: "1",
    number-align: center,
  )
  set text(lang: "ru", font: "Libertinus Serif", size: 10.5pt, hyphenate: true)
  set par(justify: true, leading: 0.62em, spacing: 0.9em)
  set heading(numbering: "1.1.")
  show heading: it => block(
    above: 1.3em, below: 0.7em,
    text(fill: accent, weight: "bold", it),
  )
  show heading.where(level: 1): it => block(
    above: 1.6em, below: 0.8em,
    text(fill: accent, size: 14pt, weight: "bold", it),
  )
  show heading.where(level: 2): set text(size: 12pt)
  show heading.where(level: 3): set text(size: 11pt)
  show raw.where(block: true): it => block(
    fill: soft, inset: 8pt, radius: 3pt, width: 100%,
    text(size: 8.5pt, it),
  )
  show raw.where(block: false): it => box(
    fill: soft, inset: (x: 3pt, y: 0pt), outset: (y: 3pt), radius: 2pt,
    text(size: 9pt, it),
  )
  set table(stroke: 0.5pt + gray, inset: 6pt)
  show table.cell.where(y: 0): set text(weight: "bold")
  set math.equation(numbering: none)
  show link: set text(fill: accent)

  align(center)[
    #v(1.2cm)
    #text(size: 20pt, weight: "bold", fill: accent, title)
    #v(0.3cm)
    #text(size: 12pt, subtitle)
    #v(0.2cm)
    #text(size: 10pt, style: "italic", source)
    #v(0.8cm)
    #line(length: 60%, stroke: 0.8pt + accent)
    #v(0.6cm)
  ]

  outline(title: [Содержание], depth: 2, indent: 1em)
  pagebreak()

  body
}

// Выделенный блок: определение, важное замечание, ответ.
#let note(title: none, body) = block(
  width: 100%, inset: 9pt, radius: 3pt,
  fill: soft, stroke: (left: 2.5pt + accent),
  {
    if title != none { text(weight: "bold", fill: accent, title); linebreak() }
    body
  },
)

// Вопрос на защите + ответ.
#let qa(question, answer) = block(
  width: 100%, below: 1.0em,
  {
    text(weight: "bold", fill: accent)[?~ #question]
    linebreak()
    answer
  },
)

// Двухколоночная таблица "термин -- пояснение".
#let deftable(..rows) = table(
  columns: (auto, 1fr),
  ..rows.pos(),
)