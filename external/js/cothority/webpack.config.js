const path = require("path");

module.exports = {
    entry: ["@babel/polyfill", "./src/index.ts"],
    output: {
      filename: "bundle.min.js",
      path: path.resolve(__dirname, "dist"),
      library: "cothority",
      libraryTarget: "umd"
    },
    module: {
      rules: [
        {
          test: /\.js$/,
          exclude: /node_modules/,
          use: {
            loader: "babel-loader",
            options: {
              presets: ["@babel/preset-env"]
            }
          }
        },
        {
            test: /\.ts$/,
            exclude: /node_modules/,
            use: [
                {
                    loader: 'babel-loader',
                    options: {
                        presets: ['@babel/preset-env'],
                    }
                },
                "ts-loader",
            ]
        }
      ]
    },
    resolve: {
        extensions: ['.js', '.ts'],
    }
  };