//go:build kyse

package docs

import (
	swagger "github.com/hyz-is/arandu-swagger"
)

@go
type SwaggerViewData = swagger.SwaggerViewData
@endgo

{{--
Layout padrão dedicado do Swagger UI.
Para utilizar dentro do layout principal da sua aplicação (ex: layouts.app):

@extends('layouts.app')

@section('content')
<div class="business-dashboard space-y-6">
	@include('docs.header')

	<div class="business-panel overflow-hidden p-0 border border-border rounded-lg bg-card">
		@include('docs.container')
	</div>
</div>
@endsection
--}}

<!doctype html>
<html lang="{{ .Locale }}" class="dark" data-theme="dark">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{ .Title }}</title>
	<link rel="icon" href="{{ .FaviconOrDefault() }}">
</head>
<body class="dark-theme">
	@include('docs.topbar')

	@include('docs.container')
</body>
</html>
