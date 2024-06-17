document.addEventListener('alpine:initializing', () => {
    Alpine.store('app', {
        theme: Alpine.$persist('light').as('theme'),
        fixed: false,
        url: __go.url,
        next: __go.next,
        data: __go.data,
        loadedMarked: false,
    })
    document.addEventListener('scroll', function () {
        const pageOffset = 20
        Alpine.store('app').fixed = (window.pageYOffset || document.documentElement.scrollTop) > pageOffset
    })

    if (Alpine.store('app').data.length > 0 && !Alpine.store('app').loadedMarked) {
        var script = document.createElement('script')
        script.src = 'https://cdn.jsdelivr.net/npm/marked/marked.min.js'
        document.head.appendChild(script)
        script.onload = function () {
            const renderer = new marked.Renderer()
            const originalTableRenderer = renderer.table.bind(renderer)
            renderer.table = (header, body) => {
                const table = originalTableRenderer(header, body)
                return `<div class="overflow-auto">${table}</div>`
            }
            marked.setOptions({ renderer })
            Alpine.store('app').loadedMarked = true
        }
    }
})