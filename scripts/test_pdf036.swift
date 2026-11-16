// Native PDF rendering and text-position check; only generated local test PDFs.
import AppKit
import Foundation
import PDFKit

var args = Array(CommandLine.arguments.dropFirst())
let copyOnly = args.first == "--copy-only"
if copyOnly { args.removeFirst() }
guard args.count >= 2 else { fatalError("Usage: test_pdf036.swift output-directory report.pdf …") }
let output = URL(fileURLWithPath: args[0], isDirectory: true)
try FileManager.default.createDirectory(at: output, withIntermediateDirectories: true)
func invalidCopy036d(_ text: String) -> Bool {
    let forbidden = #"\b(?:undefined|NaN|TODO)\b|\[object Object\]|\{\{|\$\{|(?m:^\s*null\s*$)|:\s*null\s*(?:$|\n|[,;}\]])|(?m:^\s*Fachbetrieb(?:\s*/\s*Unternehmen)?\s*:?\s*$)"#
    return text.range(of: forbidden, options: .regularExpression) != nil
}
for (text, expected) in [("Ohne Verbindung fällt die Leistung nicht auf null.", false), ("Für Elektro-Fachbetriebe.", false), ("null", true), ("Messwert: null", true), ("Fachbetrieb", true), ("Version undefined gespeichert.", true)] {
    let observed = invalidCopy036d(text)
    guard observed == expected else { fputs("036d PDF copy counterexample rejected/missed: \(text)\n", stderr); exit(1) }
}
var checked = 0
for file in args.dropFirst() {
    guard let document = PDFDocument(url: URL(fileURLWithPath: file)) else { fatalError("Generated PDF cannot be read") }
    let stem = URL(fileURLWithPath: file).deletingPathExtension().lastPathComponent
    try (document.string ?? "").write(to: output.appendingPathComponent("\(stem)-text.txt"), atomically: true, encoding: .utf8)
    let fullText = document.string ?? ""
    guard !invalidCopy036d(fullText), fullText.contains("Firma") else { fatalError("PDF has placeholders/legacy company label or lacks Firma: \(stem)") }
    if copyOnly { checked += 1; continue }
    var charts = 0
    for index in 0..<document.pageCount {
        guard let page = document.page(at: index), let text = page.string, text.contains("Rot gestrichelt: Messausfall.") else { continue }
        let bounds = page.bounds(for: .mediaBox)
        guard text.contains("W"), !text.contains("kW"), !text.contains("Treppenlinie"), !text.contains("jeder Messwert gilt"), !text.contains("linear verbunden") else { fatalError("Chart units/interpolation description missing") }
        guard let note = document.findString("Rot gestrichelt: Messausfall.", withOptions: []).first(where: { $0.pages.contains(where: { $0 === page }) }) else { fatalError("Chart note missing") }
        let noteBox = note.bounds(for: page)
        guard let connection = document.findString("Verbindungsleiste: Grau = verbunden; Hell = getrennt oder nicht beobachtet.", withOptions: []).first(where: { $0.pages.contains(where: { $0 === page }) }) else { fatalError("Separate connection legend missing") }
        let connectionBox = connection.bounds(for: page)
        guard bounds.contains(connectionBox), bounds.contains(noteBox), !noteBox.intersects(connectionBox), abs(noteBox.minY - connectionBox.minY) >= 10 else { fatalError("PDF legend text overlaps or clips") }
        if text.contains("Übung: simulierte Werte.") {
            guard let practice = document.findString("Übung: simulierte Werte.", withOptions: []).first(where: { $0.pages.contains(where: { $0 === page }) }), bounds.contains(practice.bounds(for: page)), !practice.bounds(for: page).intersects(connectionBox) else { fatalError("Practice legend clipped or overlapping") }
        }
        let noteY = noteBox.minY
        let labels = document.findString("Verbindung", withOptions: []).filter { $0.pages.contains(where: { $0 === page }) }
        guard let label = labels.min(by: { abs($0.bounds(for: page).minY - noteY - 45) < abs($1.bounds(for: page).minY - noteY - 45) }) else { fatalError("Connection label missing") }
        let box = label.bounds(for: page)
        guard box.width > 10, box.height > 3, bounds.contains(box), (30...60).contains(box.minY - noteY) else { fatalError("Chart label clipped or outside its legend row") }
        // The Go geometry regression checks this label against the filled band.
        // Also render the actual document, so fonts, paths and raster output are exercised.
        let image = page.thumbnail(of: NSSize(width: bounds.width * 2, height: bounds.height * 2), for: .mediaBox)
        guard let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff), bitmap.pixelsWide > 1000,
              let png = bitmap.representation(using: .png, properties: [:]), png.count > 10000 else { fatalError("Chart render is empty or too small") }
        let name = URL(fileURLWithPath: file).deletingPathExtension().lastPathComponent
        try png.write(to: output.appendingPathComponent("\(name)-chart-\(index + 1).png"))
        let positions: [String: Any] = ["page": index + 1, "label": "Verbindung", "x": box.minX, "y": box.minY, "width": box.width, "height": box.height, "imageWidth": bitmap.pixelsWide, "imageHeight": bitmap.pixelsHigh]
        try JSONSerialization.data(withJSONObject: positions, options: [.prettyPrinted, .sortedKeys]).write(to: output.appendingPathComponent("\(name)-chart-positions.json"))
        charts += 1
    }
    guard charts > 0 else { fatalError("No measured chart in generated PDF") }
    checked += 1
}
print(copyOnly ? "036c PDF copy: \(checked) actual documents, no placeholders/Fachbetrieb, Firma present" : "036 PDF: \(checked) generated reports rendered; chart labels inside page, W axes and interpolation description present")
