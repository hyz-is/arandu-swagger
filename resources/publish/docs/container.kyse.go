//go:build kyse

package docs

import (
	swagger "github.com/hyz-is/arandu-swagger"
)

@go
type ContainerViewData = swagger.SwaggerViewData
@endgo

<div id="swagger-ui" class="perata-swagger-container" hx-boost="false"></div>

<link rel="stylesheet" href="{{ .UIPath }}/assets/5.32.14/swagger-ui.css">
<link rel="stylesheet" href="{{ .UIPath }}/theme.css">
<script src="{{ .UIPath }}/assets/5.32.14/swagger-ui-bundle.js"></script>
<script src="{{ .UIPath }}/swagger-initializer.js"></script>
