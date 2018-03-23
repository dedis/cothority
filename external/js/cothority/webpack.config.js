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
            presets: ["stage-3"]
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
  target: "web",
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
            presets: ["stage-3"]
          }
        }
      }
    ]
  },
  externals: ["bufferutil", "utf-8-validate"],
  plugins: [new UglifyJsPlugin()]
};

module.exports = [nodeConfig, browserConfig];
