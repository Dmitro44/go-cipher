#let title-page(body) = {
  set page(
    numbering: none,
    footer: none,
  )
  align(center, body)
}

#title-page[
  Министерство образования Республики Беларусь
  #v(0.4em)
  Учреждение образования «Белорусский государственный университет \ информатики и радиоэлектроники»
  #v(2.5em)

  Факультет компьютерных систем и сетей

  Кафедра Информатики

  Дисциплина «Методы защиты информации»
  #v(4em)

  #text(weight: "bold")[ОТЧЁТ]

  к лабораторной работе №3

  на тему

  #upper[#text(weight: "bold")["Асимметричная криптография.\ Криптосистема Рабина"]]
  // #v(1em)

  БГУИР 6-05-0612-02 21

  #v(6em)

  #align(right)[
    #block(width: 10.5cm)[
      #align(left)[Выполнил студент группы 353503]

      #align(left)[СЕБЕЛЕВ Дмитрий Юрьевич]

      #v(1.5em)

      #block(width: 88%)[
        #stack(
          dir: ttb, // top-to-bottom
          spacing: 9pt,
          line(length: 100%),
          align(center, text(size: 8pt)[(дата, подпись студента)]),
        )
      ]
      #v(1em)

      #align(left)[Проверил ассистент каф. информатики]

      #align(left)[ПОЛЯКОВ Кирилл Борисович]

      #v(1.5em)

      #block(width: 88%)[
        #stack(
          dir: ttb,
          spacing: 9pt,
          line(length: 100%),
          align(center, text(size: 8pt)[(дата, подпись преподавателя)]),
        )
      ]
    ]
  ]

  #v(1fr)
  Минск 2026
]
