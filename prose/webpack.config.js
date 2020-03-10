const path = require('path')

module.exports = {
	entry: {
		entry: __dirname + '/prose.js'
	},
	output: {
		filename: 'prose.bundle.js',
		path: path.resolve(__dirname, 'dist')
	},
	module: {
		rules: [
			{
				test: /\.js$/,
				exclude: /(nodue_modules|bower_components)/,
				use: {
					loader: 'babel-loader',
					options: {
						presets: ['@babel/preset-env']
					}
				}
			}
		]
	}
}
