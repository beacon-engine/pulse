use serde::Serialize;

#[derive(Serialize)]
pub struct ContainerEvent {
    pub id: String,
    pub container_id: String,
    pub container_name: String,
    pub event: String,
    pub occurred_at: String,
}

#[derive(Serialize, Clone)]
pub struct Environment {
    pub id: String,
    pub name: String,
    pub docker_host: String,
    pub is_local: bool,
    pub created_at: String,
}

#[derive(Serialize, Clone)]
pub struct SavedStack {
    pub id: String,
    pub host_id: String,
    pub name: String,
    pub content: String,
    pub status: String,
    pub created_at: String,
    pub updated_at: String,
}
