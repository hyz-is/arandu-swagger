//go:build kyse

package docs

import (
	"github.com/arandu-io/kyse/icons"
	swagger "github.com/hyz-is/arandu-swagger"
)

@go
type TopbarViewData = swagger.SwaggerViewData
@endgo

<header class="{{ .ClassPrefixOrDefault() }}-topbar">
	<div class="{{ .ClassPrefixOrDefault() }}-topbar-wrapper">
		<div class="{{ .ClassPrefixOrDefault() }}-topbar-start">
			<a
				href="{{ .HomeURLOrDefault() }}"
				class="{{ .ClassPrefixOrDefault() }}-logo-link"
				@if(.LogoTarget != "")
					target="{{ .LogoTarget }}"
				@endif
			>
				@if(.LogoURLOrDefault() != "")
					<img src="{{ .LogoURLOrDefault() }}" alt="{{ .BrandOrDefault() }}" class="{{ .ClassPrefixOrDefault() }}-logo">
				@endif
				<span class="{{ .ClassPrefixOrDefault() }}-title">{{ .BrandOrDefault() }}</span>
			</a>
		</div>
		<div class="{{ .ClassPrefixOrDefault() }}-topbar-end">
			@if(.BackURLOrDefault() != "")
				<a
					href="{{ .BackURLOrDefault() }}"
					class="{{ .ClassPrefixOrDefault() }}-back-link"
					@if(.BackTarget != "")
						target="{{ .BackTarget }}"
					@endif
				>
					{!! icons.ArrowLeft(icons.Props{}) !!}
					<span>{{ .BackTextOrDefault() }}</span>
				</a>
			@endif
			@if(!.DisableThemeToggle)
				<button type="button" class="{{ .ClassPrefixOrDefault() }}-theme-toggle" aria-label="{{ .Translate("Toggle theme") }}" title="{{ .Translate("Toggle theme") }}">
					<span class="{{ .ClassPrefixOrDefault() }}-glyph-light" aria-hidden="true">{!! icons.Sun(icons.Props{}) !!}</span>
					<span class="{{ .ClassPrefixOrDefault() }}-glyph-dark" aria-hidden="true">{!! icons.Moon(icons.Props{}) !!}</span>
				</button>
			@endif
		</div>
	</div>
</header>
