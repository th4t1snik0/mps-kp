package schgen

// Project — schematic.kicad_pro рядом со схемой: настройки ERC и нумерация частей
// корпуса «DD1.1, DD1.2» (разделитель «.», первая часть «1») вместо «DD1A».
// Отключено то, что для учебной схемы не ошибка:
//   - lib_symbol_issues / footprint_link_issues — символы встроены в схему, библиотека mps не подключена;
//   - net_not_bus_member — шины на листе графические (как в принятой схеме), связи идут по меткам;
//   - missing_unit / missing_input_pin / missing_power_pin — неиспользуемые вентили по ГОСТ не рисуют,
//     их входы и питание описаны в «Примечании».
const Project = `{
  "erc": {
    "rule_severities": {
      "lib_symbol_issues": "ignore",
      "lib_symbol_mismatch": "ignore",
      "footprint_link_issues": "ignore",
      "net_not_bus_member": "ignore",
      "missing_unit": "ignore",
      "missing_input_pin": "ignore",
      "missing_power_pin": "ignore"
    }
  },
  "schematic": {
    "drawing": {
      "text_offset_ratio": 0.08,
      "label_size_ratio": 0.25
    },
    "subpart_id_separator": 46,
    "subpart_first_id": 49
  },
  "meta": {
    "filename": "schematic.kicad_pro",
    "version": 3
  }
}
`
