//go:build kyse

package docs

import (
	"github.com/arandu-io/kyse/icons"
	swagger "github.com/hyz-is/arandu-swagger"
)

@go
type SwaggerViewData = swagger.SwaggerViewData
@endgo

<!doctype html>
<html lang="{{ .Locale }}" class="dark" data-theme="dark">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{ .Title }}</title>
	<link rel="icon" href="/favicon.ico">
	<link rel="stylesheet" href="{{ .UIPath }}/assets/5.32.14/swagger-ui.css">
	<link rel="stylesheet" href="{{ .UIPath }}/theme.css">
</head>
<body class="dark-theme">
	<header class="arandu-swagger-topbar">
		<div class="arandu-swagger-topbar-wrapper">
			<div class="arandu-swagger-topbar-start">
				<a href="/" class="arandu-swagger-logo-link">
					<img src="/favicon.svg" alt="Peráta" class="arandu-swagger-logo">
					<span class="arandu-swagger-title">Peráta</span>
				</a>
			</div>
			<div class="arandu-swagger-topbar-end">
				<a href="/" class="arandu-swagger-back-link">
					{!! icons.ArrowLeft(icons.Props{}) !!}
					<span>Voltar para o site</span>
				</a>
				<button type="button" class="arandu-swagger-theme-toggle" aria-label="Alternar tema" title="Alternar tema">
					<span class="arandu-swagger-glyph-light" aria-hidden="true">{!! icons.Sun(icons.Props{}) !!}</span>
					<span class="arandu-swagger-glyph-dark" aria-hidden="true">{!! icons.Moon(icons.Props{}) !!}</span>
				</button>
			</div>
		</div>
	</header>

	<div id="swagger-ui" class="perata-swagger-container" hx-boost="false"></div>

	<script src="{{ .UIPath }}/assets/5.32.14/swagger-ui-bundle.js"></script>
	<script src="{{ .UIPath }}/swagger-initializer.js"></script>
</body>
</html>
