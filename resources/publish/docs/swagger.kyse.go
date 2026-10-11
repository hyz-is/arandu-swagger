//go:build kyse

package docs

import (
	swagger "github.com/hyz-is/arandu-swagger"
)

@go
type SwaggerViewData = swagger.SwaggerViewData
@endgo

{{--
The documentation page on its own, with the package's topbar.
To draw it inside the application's main layout instead (for example
layouts.app), replace the document below with:

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
