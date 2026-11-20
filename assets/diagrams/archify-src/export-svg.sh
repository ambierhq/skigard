#!/bin/bash
# usage: export.sh <html> <format svg-dark|svg-light> <out>
B="/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
python3 - "$1" "$2" <<'P'
import sys
h=open(sys.argv[1]).read()
s='''<script>
(function(){var cap=null;var oc=URL.createObjectURL;URL.createObjectURL=function(b){cap=b;return oc.call(URL,b)};
window.addEventListener('load',function(){setTimeout(function(){
var m=document.querySelector('button[data-format="%s"]');
if(!m){document.body.setAttribute('data-x','nobtn');return}
m.click();var t=setInterval(function(){if(cap){clearInterval(t);cap.text().then(function(x){var p=document.createElement('pre');p.id='OUT';p.textContent=x;document.body.appendChild(p)})}},100)},1500)});})();
</script></body>''' % sys.argv[2]
open('.archify/_exp.html','w').write(h.replace('</body>',s))
P
"$B" --headless --virtual-time-budget=8000 --dump-dom file://$PWD/.archify/_exp.html 2>/dev/null > .archify/_dom.html
python3 - "$3" <<'P'
import re,sys,html
d=open('.archify/_dom.html').read()
m=re.search(r'<pre id="OUT">(.*?)</pre>',d,re.S)
print('found',bool(m))
if m: open(sys.argv[1],'w').write(html.unescape(m.group(1)))
P
