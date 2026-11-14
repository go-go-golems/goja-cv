// 05-pdftext: print a PDF's text with macOS PDFKit (no poppler here). Run: swift 05-pdftext.swift FILE.pdf
import PDFKit
let d = PDFDocument(url: URL(fileURLWithPath: CommandLine.arguments[1]))!
print(d.string ?? "")
