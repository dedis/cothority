const path = require("path");
const UglifyJsPlugin = require("uglifyjs-webpack-plugin");
const nodeExternals = require("webpack-node-externals");

const nodeConfig = {
  target: "node",
  entry: "./index.js",
  output: {
    filename: "bundle.node.min.js",
    path: path.resolve(__dirname, "dist"),
    libraryTarget: "commonjs2"
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: /(node_modules|bower_components)/,
        use: {
          loader: "babel-loader",
          options: {
            presets: [["env", { targets: { node: 8 } }]],
            plugins: [require("babel-plugin-transform-object-rest-spread")]
          }
        }
      }
    ]
  },
  externals: [nodeExternals()],
  plugins: [new UglifyJsPlugin()]
};

const browserConfig = {
  entry: "./index.js",
  output: {
    filename: "bundle.min.js",
    path: path.resolve(__dirname, "dist"),
    library: "cothority",
    libraryTarget: "umd"
  },
  resolve: {
    alias: {
      ws: path.resolve(__dirname, "lib", "shims", "ws.js")
    }
  },
  module: {
    rules: [
      {
        test: /\.js$/,
        exclude: /(node_modules|bower_components)/,
        use: {
          loader: "babel-loader",
          options: {
            presets: [
              ["env", { targets: { browsers: [">1%"] }, useBuiltIns: true }]
            ],
            plugins: [require("babel-plugin-transform-object-rest-spread")]
          }
        }
      }
    ]
  },
  externals: ["bufferutil", "utf-8-validate"],
  plugins: [new UglifyJsPlugin()]
};

module.exports = [nodeConfig, browserConfig];
