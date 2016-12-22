'use strict';

var fs = require('fs')
  , pkg = require('./package.json')

  , gulp = require('gulp')
  , header = require('gulp-header')
  , footer = require('gulp-footer')
  , concat = require('gulp-concat')
  , uglify = require('gulp-uglify')
  , jshint = require('gulp-jshint')
  , sourcemaps = require('gulp-sourcemaps');


var paths = {
  vendor: [
  "grid.js", "version.js", "detector.js", "formatinf.js",
  "errorlevel.js", "bitmat.js", "datablock.js","bmparser.js",
  "datamask.js","rsdecoder.js","gf256poly.js", "gf256.js",
  "decoder.js", "qrcode.js", "findpat.js", "alignpat.js",
  "databr.js"]
    .map(function (file) { return 'vendor/' + file; }),

  vendorBuild: ['build/qrcode.js'],

  src: ['src/qcode-decoder.js']
};


gulp.task('build', ['build:vendor'], function() {
  return gulp.src(paths.vendorBuild.concat(paths.src))
    .pipe(sourcemaps.init())
      .pipe(concat('qcode-decoder.min.js'))
      .pipe(uglify({mangle: true}))
    .pipe(sourcemaps.write('./'))
    .pipe(gulp.dest('build'));
});

gulp.task('build:vendor', function () {
  var vendor = {
    header: fs.readFileSync(__dirname + '/vendor/header-umd.js'),
    footer: fs.readFileSync(__dirname + '/vendor/footer-umd.js')
  };

  return gulp.src(paths.vendor)
    .pipe(sourcemaps.init())
      .pipe(concat('qrcode.js'))
      .pipe(header(vendor.header))
      .pipe(footer(vendor.footer))
    .pipe(sourcemaps.write('./'))
    .pipe(gulp.dest('build'));
});

gulp.task('hint', function () {
  return gulp.src('src/*.js')
    .pipe(jshint())
    .pipe(jshint.reporter('default'))
    .pipe(jshint.reporter('fail'));
});

gulp.task('watch', function () {
  gulp.watch(paths.src, ['hint', 'build']);
});

gulp.task('default', ['hint', 'build']);
