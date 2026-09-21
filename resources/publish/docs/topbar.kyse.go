//go:build kyse

package docs

import (
	"github.com/arandu-io/kyse/icons"
	swagger "github.com/hyz-is/arandu-swagger"
)

@go
type SwaggerViewData = swagger.SwaggerViewData
@endgo

<header class="arandu-swagger-topbar">
	<div class="arandu-swagger-topbar-wrapper">
		<div class="arandu-swagger-topbar-start">
			<a href="{{ .HomeURLOrDefault() }}" class="arandu-swagger-logo-link" @if(.LogoTarget != "")target="{{ .LogoTarget }}"@endif>
				<img src="{{ .LogoURLOrDefault() }}" alt="{{ .BrandOrDefault() }}" class="arandu-swagger-logo">
				<span class="arandu-swagger-title">{{ .BrandOrDefault() }}</span>
			</a>
		</div>
		<div class="arandu-swagger-topbar-end">
			@if(.BackURLOrDefault() != "")
				<a href="{{ .BackURLOrDefault() }}" class="arandu-swagger-back-link" @if(.BackTarget != "")target="{{ .BackTarget }}"@endif>
					{!! icons.ArrowLeft(icons.Props{}) !!}
					<span>{{ .BackTextOrDefault() }}</span>
				</a>
			@endif
			@if(!.DisableThemeToggle)
				<button type="button" class="arandu-swagger-theme-toggle" aria-label="Alternar tema" title="Alternar tema">
					<span class="arandu-swagger-glyph-light" aria-hidden="true">{!! icons.Sun(icons.Props{}) !!}</span>
					<span class="arandu-swagger-glyph-dark" aria-hidden="true">{!! icons.Moon(icons.Props{}) !!}</span>
				</button>
			@endif
		</div>
	</div>
</header>
