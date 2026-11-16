// macOS-only browser harness using the installed system WebKit. No HTTP/LAN.
// swift -module-cache-path artifacts/swift-cache scripts/test_webui.swift [--wide] [--file-harness] [--screenshots] [--motion] [--dark] [--reduced-motion]
import Foundation
import AppKit
import WebKit

final class Load: NSObject, WebFrameLoadDelegate {
    var done = false
    func webView(_ sender: WebView!, didFinishLoadFor frame: WebFrame!) {
        if frame == sender.mainFrame { done = true }
    }
}
let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
func read(_ path: String) throws -> String { try String(contentsOf: root.appendingPathComponent(path), encoding: .utf8) }
func script(_ path: String) throws -> String { try read(path).replacingOccurrences(of: "</script", with: "<\\/script") }
let app = NSApplication.shared
app.setActivationPolicy(.prohibited)
let artifacts = root.appendingPathComponent("artifacts")
try FileManager.default.createDirectory(at: artifacts, withIntermediateDirectories: true)
var html = try read("internal/webui/assets/index.html")
html = html.replacingOccurrences(of: "{{.Token}}", with: "synthetic-test-token").replacingOccurrences(of: "{{.Version}}", with: "test-build")
html = html.replacingOccurrences(of: "<link rel=\"stylesheet\" href=\"/style.css\">", with: "<style>\(try read("internal/webui/assets/style.css"))</style>")
for name in ["qr-vendor.js", "input.js", "i18n-data.js", "i18n.js", "result.js", "guide.js", "workspace.js", "profile.js", "app.js"] {
 html = html.replacingOccurrences(of: "<script src=\"/\(name)\" defer></script>", with: "")
}
let motion = CommandLine.arguments.contains("--motion")
let reduced = CommandLine.arguments.contains("--reduced-motion")
// Legacy offscreen WebView suspends visibility/rAF/timeline; drive these clocks locally.
// Rendering, keyframes, DOM and APIs remain the actual system WebKit. No FPS benchmark.
let preference = motion ? """
<script>
Object.defineProperty(document,'hidden',{get:()=>false});
window.requestAnimationFrame=fn=>setTimeout(()=>fn(performance.now()),16);
window.cancelAnimationFrame=id=>clearTimeout(id);
const effectOrigins=new WeakMap();
setInterval(()=>document.getAnimations().forEach(a=>{
  if(!effectOrigins.has(a))effectOrigins.set(a,performance.now());
  a.currentTime=performance.now()-effectOrigins.get(a);
}),16);
const nativeMatchMedia=window.matchMedia.bind(window);
let reducedTest=\(reduced);
const motionListeners=[];
window.matchMedia=q=>q==='(prefers-reduced-motion: reduce)'?{
  get matches(){return reducedTest;},addEventListener:(_,fn)=>motionListeners.push(fn)
}:nativeMatchMedia(q);
window.setTestReducedMotion=value=>{reducedTest=value;motionListeners.forEach(fn=>fn({matches:value}));};
</script>
""" : ""
let text034 = CommandLine.arguments.contains("--text034")
let flow032 = CommandLine.arguments.contains("--flow032")
let flow030 = CommandLine.arguments.contains("--flow030")
let flow029 = CommandLine.arguments.contains("--flow029")
let flow025c = CommandLine.arguments.contains("--flow025c")
let flow025 = flow025c || CommandLine.arguments.contains("--flow025")
let fileHarness = CommandLine.arguments.contains("--file-harness")
let flow024c = CommandLine.arguments.contains("--flow024c")
let flow024b = flow024c || CommandLine.arguments.contains("--flow024b")
let flow024 = flow024b || CommandLine.arguments.contains("--flow024")
let flow023b = CommandLine.arguments.contains("--flow023b")
let flow023 = flow023b || CommandLine.arguments.contains("--flow023")
let flow022 = flow025 || flow024 || flow023 || CommandLine.arguments.contains("--flow022")
let flow020b = CommandLine.arguments.contains("--flow020b")
let flow020 = flow020b || CommandLine.arguments.contains("--flow020")
let flow019b = CommandLine.arguments.contains("--flow019b")
let flow019 = flow022 || flow020 || flow019b || CommandLine.arguments.contains("--flow019")
let logoFixtures024b = flow024b ? "<script>window.logoFixtures024b="+(try read("scripts/fixtures/logo024b.json"))+";</script>" : ""
let qrFixtures019 = flow019 ? "<script>window.qrImageFixtures019="+(try read("scripts/fixtures/qr019.json"))+";</script>" : ""
let flow016b = CommandLine.arguments.contains("--flow016b")
let flow016 = CommandLine.arguments.contains("--flow016")
let flow015 = CommandLine.arguments.contains("--flow015")
let suite = text034 ? "scripts/test_text034_flow.js" : flow032 ? "scripts/test_flow032.js" : flow030 ? "scripts/test_flow030.js" : flow029 ? "scripts/test_flow029.js" : flow025 ? "scripts/test_flow025.js" : flow024b ? "scripts/test_flow024b.js" : flow024 ? "scripts/test_flow024.js" : flow023 ? "scripts/test_flow023.js" : flow022 ? "scripts/test_flow022.js" : flow020 ? "scripts/test_flow020.js" : flow019b ? "scripts/test_flow019b.js" : flow019 ? "scripts/test_flow019.js" : flow016b ? "scripts/test_flow016b.js" : flow016 ? "scripts/test_flow016.js" : flow015 ? "scripts/test_flow015.js" : motion ? "scripts/test_motion.js" : "scripts/test_webui.js"
html = html.replacingOccurrences(of: "</body>", with: preference + logoFixtures024b + qrFixtures019 + "<script>\(try script("scripts/test_webui_mock.js"))</script><script>\(try script("internal/webui/assets/qr-vendor.js"))</script><script>\(try script("internal/webui/assets/input.js"))</script><script>\(try script("internal/webui/assets/i18n-data.js"))</script><script>\(try script("internal/webui/assets/i18n.js"))</script><script>\(try script("internal/webui/assets/result.js"))</script><script>\(try script("internal/webui/assets/guide.js"))</script><script>\(try script("internal/webui/assets/workspace.js"))</script><script>\(try script("internal/webui/assets/profile.js"))</script><script>\(try script("internal/webui/assets/app.js"))</script><script>\(try script(suite))</script></body>")
let dark = CommandLine.arguments.contains("--dark")
if dark { html = html.replacingOccurrences(of: "@media(prefers-color-scheme:dark)", with: "@media all") }
func pump(_ seconds: Double) { RunLoop.current.run(mode: .default, before: Date().addingTimeInterval(seconds)) }
let sizes = flow032 ? [("1440",1440.0),("390",390.0)] : flow030 ? [("1440",1440.0),("1280",1280.0),("1100",1100.0),("1099",1099.0),("900",900.0),("700",700.0),("601",601.0),("600",600.0),("390",390.0)] : flow029 || flow025 || flow024 ? [("1440",1440.0),("390",390.0)] : CommandLine.arguments.contains("--zoom200") ? [("zoom200",720.0)] : flow019 ? [("1440",1440.0),("1280",1280.0),("1024",1024.0),("390",390.0)] : CommandLine.arguments.contains("--wide") ? [("1440",1440.0),("390",390.0)] : [("desktop",1100.0),("mobil",390.0)]
let harnessFile=artifacts.appendingPathComponent("stromea-webui-\(UUID().uuidString).html")
try html.write(to:harnessFile,atomically:true,encoding:.utf8)
let server=Process(),serverOutput=Pipe()
server.executableURL=URL(fileURLWithPath:"/usr/bin/env")
server.arguments=["python3",root.appendingPathComponent("scripts/webui_test_server.py").path,harnessFile.path]
server.standardOutput=serverOutput
if !fileHarness { try server.run() }
var portData=Data()
while !fileHarness {let b=serverOutput.fileHandleForReading.readData(ofLength:1);if b.isEmpty || b==Data([10]){break};portData.append(b)}
guard let origin = fileHarness ? harnessFile : URL(string:"http://127.0.0.1:\(String(data:portData,encoding:.utf8) ?? "")/") else{fatalError("loopback harness did not start")}
defer{if server.isRunning {server.terminate()};try? FileManager.default.removeItem(at:harnessFile)}
for (label, width) in sizes {
    let loader = Load()
    let view = WebView(frame: NSRect(x: 0, y: 0, width: width, height: label == "1280" ? 800 : 900))
    // A view hosted in an offscreen window has a real scrolling/compositing root.
    // No window is ordered onscreen and no network is used.
    let host = NSWindow(contentRect: view.frame, styleMask: [], backing: .buffered, defer: false)
    host.contentView = view
    view.frameLoadDelegate = loader
    view.preferences.isJavaScriptEnabled = true
    view.mainFrame.load(URLRequest(url:origin))
    let deadline = Date().addingTimeInterval(30)
    while Date() < deadline && view.stringByEvaluatingJavaScript(from: "window.uiTest && window.uiTest.done ? 'done' : ''") != "done" { pump(0.05) }
    let error = view.stringByEvaluatingJavaScript(from: "window.uiTest ? window.uiTest.error : 'JavaScript did not start'") ?? "Unknown error"
    guard error.isEmpty, view.stringByEvaluatingJavaScript(from: "String(window.uiTest.done)") == "true" else {
        if server.isRunning {server.terminate()};fputs("FAIL \(label): \(error)\n", stderr); print(view.stringByEvaluatingJavaScript(from: "JSON.stringify({route:location.href,local:document.querySelector('.local-page')?.innerHTML,profile:document.getElementById('profile-business')?.outerHTML,nav:document.querySelector('.main-menu').outerHTML,hidden:document.hidden,viewport:{width:innerWidth,height:innerHeight},overflow:[...document.querySelectorAll('body *')].filter(n=>n.getBoundingClientRect().right>innerWidth+1).slice(0,15).map(n=>[n.tagName,n.id,n.className,n.getBoundingClientRect().right]),pairBottom:document.getElementById('pair')?.getBoundingClientRect().bottom,value:document.getElementById('power-value')?.textContent})") ?? ""); exit(1)
    }
    if motion || flow015 || flow016 { print("Contrast \(label): \(view.stringByEvaluatingJavaScript(from: "JSON.stringify(window.uiTest.contrasts)") ?? "{}")") }
    print("PASS \(label): \(view.stringByEvaluatingJavaScript(from: "String(window.uiTest.checks)") ?? "?") browser assertions")
    if flow030 { print("Connect bottom \(label): \(view.stringByEvaluatingJavaScript(from: "String(window.uiTest.connectBottom)") ?? "?") px") }
    if flow024b, let logo=view.stringByEvaluatingJavaScript(from:"window.uiTest.profileLogo"),let comma=logo.firstIndex(of:","),let data=Data(base64Encoded:String(logo[logo.index(after:comma)...])) {try data.write(to:artifacts.appendingPathComponent("024b-profil-logo.png"))}
    if CommandLine.arguments.contains("--screenshots") {
        let json = view.stringByEvaluatingJavaScript(from: "JSON.stringify(window.uiTest.snapshots)")!
        let snapshots = try JSONSerialization.jsonObject(with: json.data(using: .utf8)!) as! [String: String]
        view.preferences.isJavaScriptEnabled = false
        for name in snapshots.keys.sorted() {
            if flow019 && !flow020 && !flow022 {
                let selected: Set<String> = label == "1440" ? ["einstieg","verbindet","vertraut","abgelehnt","falsche-ski","qr-ungueltig","vorbereitung","lauf-wenig-last","ergebnis-HeatPump-de","ergebnis-Inverter-en","ergebnis-ChargingStation-ru"] : label == "1280" ? ["vertraut","vorbereitung","ergebnis-Inverter-de"] : label == "1024" ? ["manuell","lauf-wenig-last","ergebnis-ChargingStation-en"] : ["manuell","qr-ungueltig","lauf-wenig-last","ergebnis-HeatPump-ru"]
                if !selected.contains(name) { continue }
            }
            loader.done = false
            view.setFrameSize(NSSize(width: width, height: label == "1280" ? 800 : 900))
            view.mainFrame.loadHTMLString(snapshots[name]!, baseURL: root)
            let deadline = Date().addingTimeInterval(10)
            while !loader.done && Date() < deadline { pump(0.05) }
            guard loader.done else { fatalError("Snapshot load timed out") }
            // Read document height through the DOM; JavaScript stays disabled.
            let height = flow022 ? (label == "1280" ? 800.0 : 900.0) : flow019 && ["einstieg","manuell","qr-ungueltig","verbindet","vertraut","abgelehnt","falsche-ski"].contains(name) ? (label == "1280" ? 800.0 : 900.0) : max(900, view.mainFrame.frameView.documentView.frame.height)
            view.setFrameSize(NSSize(width: width, height: height))
            view.layoutSubtreeIfNeeded()
            if flow022 {
                let meta = view.mainFrame.domDocument.getElementsByName("screenshot-scroll")?.item(0) as? DOMHTMLMetaElement
                let offset = Double(meta?.content ?? "0") ?? 0
                view.mainFrame.frameView.documentView.scroll(NSPoint(x: 0, y: offset))
                view.layoutSubtreeIfNeeded()
            }
            let bitmap = view.bitmapImageRepForCachingDisplay(in: view.bounds)!
            view.cacheDisplay(in: view.bounds, to: bitmap)
            let png = bitmap.representation(using: .png, properties: [:])!
            try png.write(to: artifacts.appendingPathComponent("\(flow032 ? "032" : flow030 ? "030" : flow029 ? "029" : flow025c ? "025c" : flow025 ? "025" : flow024c ? "024c" : flow024b ? "024b" : flow024 ? "024" : flow023b ? "023b" : flow023 ? "023" : flow022 ? "022" : flow020b ? "020b" : flow020 ? "020" : flow019b ? "019b" : flow019 ? "019" : flow016b ? "016b" : flow016 ? "016" : flow015 ? "015" : motion ? "014" : "011")-\(name)-\(label)\(dark ? "-dunkel" : "")\(reduced ? "-reduziert" : "").png"))
        }
    }
}

if CommandLine.arguments.contains("--screenshots"), FileManager.default.fileExists(atPath: artifacts.appendingPathComponent("009-protokoll.html").path) {
    for (label, width) in [("desktop",1100.0),("mobil",390.0)] {
        let loader=Load()
        let view=WebView(frame:NSRect(x:0,y:0,width:width,height:900))
        view.frameLoadDelegate=loader
        view.preferences.isJavaScriptEnabled=false
        view.mainFrame.loadHTMLString(try read("artifacts/009-protokoll.html"),baseURL:root)
        let deadline=Date().addingTimeInterval(10)
        while !loader.done && Date()<deadline {pump(0.05)}
        guard loader.done else {fatalError("Download preview timed out")}
        view.setFrameSize(NSSize(width:width,height:max(900,view.mainFrame.frameView.documentView.frame.height)))
        view.layoutSubtreeIfNeeded()
        let bitmap=view.bitmapImageRepForCachingDisplay(in:view.bounds)!
        view.cacheDisplay(in:view.bounds,to:bitmap)
        try bitmap.representation(using:.png,properties:[:])!.write(to:artifacts.appendingPathComponent("009-protokoll-\(label).png"))
    }
}
