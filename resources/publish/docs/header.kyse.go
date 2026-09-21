//go:build kyse

package docs

import (
	"github.com/arandu-io/kyse/icons"
	swagger "github.com/hyz-is/arandu-swagger"
)

@go
type SwaggerViewData = swagger.SwaggerViewData
@endgo

<div class="business-context">
	<div class="flex items-center gap-2 text-sm text-muted-foreground">
		<a href="{{ .HomeURLOrDefault() }}" class="hover:text-foreground transition-colors flex items-center gap-1.5">
			{!! icons.House(icons.Props{}) !!}
			<span>Workspace</span>
		</a>
		<span>/</span>
		<span class="text-foreground font-medium flex items-center gap-1.5">
			{!! icons.BookOpen(icons.Props{}) !!}
			<span>Documentação da API</span>
		</span>
	</div>
	<div class="flex items-center gap-2">
		<span class="badge border border-border bg-muted/60 text-muted-foreground text-xs px-2.5 py-0.5 rounded-md font-mono">
			OAS 3.1
		</span>
		@if(.Version != "")
			<span class="badge border border-primary/30 bg-primary/10 text-primary text-xs px-2.5 py-0.5 rounded-md font-medium">
				v{{ .Version }}
			</span>
		@endif
	</div>
</div>

<header class="business-heading">
	<div>
		<p class="onboarding-eyebrow">INTEGRAÇÃO E DESENVOLVEDOR</p>
		<h1>{{ .Title }}</h1>
		@if(.Description != "")
			<p class="max-w-3xl text-sm text-muted-foreground mt-1">{{ .Description }}</p>
		@endif
	</div>
	<div class="flex items-center gap-2.5">
		@if(.BackURLOrDefault() != "")
			<a class="btn" href="{{ .BackURLOrDefault() }}" data-variant="secondary">
				{!! icons.ArrowLeft(icons.Props{}) !!}
				<span>{{ .BackTextOrDefault() }}</span>
			</a>
		@endif
		@if(.SpecPath != "")
			<a class="btn" href="{{ .SpecPath }}" target="_blank" rel="noopener noreferrer">
				{!! icons.Code(icons.Props{}) !!}
				<span>OpenAPI JSON</span>
				{!! icons.ArrowUpRight(icons.Props{}) !!}
			</a>
		@endif
		<button type="button" class="btn" onclick="document.querySelector('.swagger-ui .btn.authorize')?.click()">
			{!! icons.Lock(icons.Props{}) !!}
			<span>Autorizar</span>
		</button>
	</div>
</header>
