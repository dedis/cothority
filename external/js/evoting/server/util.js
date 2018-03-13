const txt2dict = (txt, opt_operator) => {
  if (! opt_operator) {
    opt_operator = '=';
  }
  var dict = {};
  txt.split('\n').forEach(function (line) {
    line = line.replace(/\r$/, '');
    var sepIndex = line.indexOf(opt_operator);
    if (sepIndex == -1) {
      if (line.length)
        return;
    }
    var k = line.slice(0, sepIndex);
    var v = line.slice(sepIndex + opt_operator.length);
    dict[k] = v;
  });
  return dict;
}

const dict2txt = (data) => {
  return Object.keys(data).map(k => k + '=' + data[k] + '\n').join('').slice(0, -1)
}

module.exports = {
  txt2dict,
  dict2txt
}
