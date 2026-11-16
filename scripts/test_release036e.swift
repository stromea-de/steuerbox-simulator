import Foundation
import PDFKit
let out = CommandLine.arguments[1]
for name in ["confirmed", "unconfirmed"] {
 let url = URL(fileURLWithPath: out).appendingPathComponent(name + ".pdf")
 guard let pdf = PDFDocument(url: url), let raw = pdf.string else { fatalError("PDF unreadable: \(url)") }
 let text = raw.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ")
 let expected = name == "confirmed" ? "Die Testgrenzen wurden wieder aufgehoben." : "Aufheben der Testgrenzen gesendet, keine Bestätigung vom Gerät."
 precondition(text.components(separatedBy: expected).count == 2, "release sentence must occur exactly once: \(name)")
 precondition(!text.contains("Testergebnis aufheben"), "cleanup must not be a checked function")
 // 036f: no separate release field/section repeating the sentence.
 for repeated in ["Testgrenzen aufgehoben", "am Gerät kontrollieren", "Ja, vom Gerät bestätigt", "Freigabestatus"] {
  precondition(!text.contains(repeated), "release statement repeated in PDF: \(repeated) (\(name))")
 }
 try raw.write(to: URL(fileURLWithPath: out).appendingPathComponent(name + "-text.txt"), atomically: true, encoding: .utf8)
 print("036e PDF text: \(name) passed")
}
